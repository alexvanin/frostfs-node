package shard

import (
	"errors"
	"fmt"

	"github.com/TrueCloudLab/frostfs-node/pkg/core/object"
	"github.com/TrueCloudLab/frostfs-node/pkg/local_object_storage/blobstor"
	meta "github.com/TrueCloudLab/frostfs-node/pkg/local_object_storage/metabase"
	"github.com/TrueCloudLab/frostfs-node/pkg/local_object_storage/shard/mode"
	objectSDK "github.com/TrueCloudLab/frostfs-sdk-go/object"
	oid "github.com/TrueCloudLab/frostfs-sdk-go/object/id"
	"go.uber.org/zap"
)

func (s *Shard) handleMetabaseFailure(stage string, err error) error {
	s.log.Error("metabase failure, switching mode",
		zap.String("stage", stage),
		zap.Stringer("mode", mode.ReadOnly),
		zap.Error(err))

	err = s.SetMode(mode.ReadOnly)
	if err == nil {
		return nil
	}

	s.log.Error("can't move shard to readonly, switch mode",
		zap.String("stage", stage),
		zap.Stringer("mode", mode.DegradedReadOnly),
		zap.Error(err))

	err = s.SetMode(mode.DegradedReadOnly)
	if err != nil {
		return fmt.Errorf("could not switch to mode %s", mode.DegradedReadOnly)
	}
	return nil
}

// Open opens all Shard's components.
func (s *Shard) Open() error {
	components := []interface{ Open(bool) error }{
		s.blobStor, s.metaBase,
	}

	if s.hasWriteCache() {
		components = append(components, s.writeCache)
	}

	if s.pilorama != nil {
		components = append(components, s.pilorama)
	}

	for i, component := range components {
		if err := component.Open(false); err != nil {
			if component == s.metaBase {
				// We must first open all other components to avoid
				// opening non-existent DB in read-only mode.
				for j := i + 1; j < len(components); j++ {
					if err := components[j].Open(false); err != nil {
						// Other components must be opened, fail.
						return fmt.Errorf("could not open %T: %w", components[j], err)
					}
				}
				err = s.handleMetabaseFailure("open", err)
				if err != nil {
					return err
				}

				break
			}

			return fmt.Errorf("could not open %T: %w", component, err)
		}
	}
	return nil
}

type metabaseSynchronizer Shard

func (x *metabaseSynchronizer) Init() error {
	return (*Shard)(x).refillMetabase()
}

// Init initializes all Shard's components.
func (s *Shard) Init() error {
	type initializer interface {
		Init() error
	}

	var components []initializer

	if !s.GetMode().NoMetabase() {
		var initMetabase initializer

		if s.needRefillMetabase() {
			initMetabase = (*metabaseSynchronizer)(s)
		} else {
			initMetabase = s.metaBase
		}

		components = []initializer{
			s.blobStor, initMetabase,
		}
	} else {
		components = []initializer{s.blobStor}
	}

	if s.hasWriteCache() {
		components = append(components, s.writeCache)
	}

	if s.pilorama != nil {
		components = append(components, s.pilorama)
	}

	for _, component := range components {
		if err := component.Init(); err != nil {
			if component == s.metaBase {
				if errors.Is(err, meta.ErrOutdatedVersion) {
					return fmt.Errorf("metabase initialization: %w", err)
				}

				err = s.handleMetabaseFailure("init", err)
				if err != nil {
					return err
				}

				break
			}

			return fmt.Errorf("could not initialize %T: %w", component, err)
		}
	}

	s.updateMetrics()

	s.gc = &gc{
		gcCfg:       &s.gcCfg,
		remover:     s.removeGarbage,
		stopChannel: make(chan struct{}),
		eventChan:   make(chan Event),
		mEventHandler: map[eventType]*eventHandlers{
			eventNewEpoch: {
				cancelFunc: func() {},
				handlers: []eventHandler{
					s.collectExpiredObjects,
					s.collectExpiredTombstones,
					s.collectExpiredLocks,
				},
			},
		},
	}

	s.gc.init()

	return nil
}

func (s *Shard) refillMetabase() error {
	err := s.metaBase.Reset()
	if err != nil {
		return fmt.Errorf("could not reset metabase: %w", err)
	}

	obj := objectSDK.New()

	err = blobstor.IterateBinaryObjects(s.blobStor, func(addr oid.Address, data []byte, descriptor []byte) error {
		if err := obj.Unmarshal(data); err != nil {
			s.log.Warn("could not unmarshal object",
				zap.Stringer("address", addr),
				zap.String("err", err.Error()))
			return nil
		}

		//nolint: exhaustive
		switch obj.Type() {
		case objectSDK.TypeTombstone:
			tombstone := objectSDK.NewTombstone()

			if err := tombstone.Unmarshal(obj.Payload()); err != nil {
				return fmt.Errorf("could not unmarshal tombstone content: %w", err)
			}

			tombAddr := object.AddressOf(obj)
			memberIDs := tombstone.Members()
			tombMembers := make([]oid.Address, 0, len(memberIDs))

			for i := range memberIDs {
				a := tombAddr
				a.SetObject(memberIDs[i])

				tombMembers = append(tombMembers, a)
			}

			var inhumePrm meta.InhumePrm

			inhumePrm.SetTombstoneAddress(tombAddr)
			inhumePrm.SetAddresses(tombMembers...)

			_, err = s.metaBase.Inhume(inhumePrm)
			if err != nil {
				return fmt.Errorf("could not inhume objects: %w", err)
			}
		case objectSDK.TypeLock:
			var lock objectSDK.Lock
			if err := lock.Unmarshal(obj.Payload()); err != nil {
				return fmt.Errorf("could not unmarshal lock content: %w", err)
			}

			locked := make([]oid.ID, lock.NumberOfMembers())
			lock.ReadMembers(locked)

			cnr, _ := obj.ContainerID()
			id, _ := obj.ID()
			err = s.metaBase.Lock(cnr, id, locked)
			if err != nil {
				return fmt.Errorf("could not lock objects: %w", err)
			}
		}

		var mPrm meta.PutPrm
		mPrm.SetObject(obj)
		mPrm.SetStorageID(descriptor)

		_, err := s.metaBase.Put(mPrm)
		if err != nil && !meta.IsErrRemoved(err) && !errors.Is(err, meta.ErrObjectIsExpired) {
			return err
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("could not put objects to the meta: %w", err)
	}

	err = s.metaBase.SyncCounters()
	if err != nil {
		return fmt.Errorf("could not sync object counters: %w", err)
	}

	return nil
}

// Close releases all Shard's components.
func (s *Shard) Close() error {
	components := []interface{ Close() error }{}

	if s.pilorama != nil {
		components = append(components, s.pilorama)
	}

	if s.hasWriteCache() {
		components = append(components, s.writeCache)
	}

	components = append(components, s.blobStor, s.metaBase)

	for _, component := range components {
		if err := component.Close(); err != nil {
			return fmt.Errorf("could not close %s: %w", component, err)
		}
	}

	// If Init/Open was unsuccessful gc can be nil.
	if s.gc != nil {
		s.gc.stop()
	}

	return nil
}

// Reload reloads configuration portions that are necessary.
// If a config option is invalid, it logs an error and returns nil.
// If there was a problem with applying new configuration, an error is returned.
func (s *Shard) Reload(opts ...Option) error {
	// Do not use defaultCfg here missing options need not be reloaded.
	var c cfg
	for i := range opts {
		opts[i](&c)
	}

	s.m.Lock()
	defer s.m.Unlock()

	ok, err := s.metaBase.Reload(c.metaOpts...)
	if err != nil {
		if errors.Is(err, meta.ErrDegradedMode) {
			s.log.Error("can't open metabase, move to a degraded mode", zap.Error(err))
			_ = s.setMode(mode.DegradedReadOnly)
		}
		return err
	}
	if ok {
		var err error
		if c.refillMetabase {
			// Here we refill metabase only if a new instance was opened. This is a feature,
			// we don't want to hang for some time just because we forgot to change
			// config after the node was updated.
			err = s.refillMetabase()
		} else {
			err = s.metaBase.Init()
		}
		if err != nil {
			s.log.Error("can't initialize metabase, move to a degraded-read-only mode", zap.Error(err))
			_ = s.setMode(mode.DegradedReadOnly)
			return err
		}
	}

	s.log.Info("trying to restore read-write mode")
	return s.setMode(mode.ReadWrite)
}
