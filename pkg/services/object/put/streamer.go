package putsvc

import (
	"context"
	"errors"
	"fmt"

	"github.com/TrueCloudLab/frostfs-node/pkg/core/client"
	"github.com/TrueCloudLab/frostfs-node/pkg/core/netmap"
	"github.com/TrueCloudLab/frostfs-node/pkg/services/object/util"
	"github.com/TrueCloudLab/frostfs-node/pkg/services/object_manager/placement"
	"github.com/TrueCloudLab/frostfs-node/pkg/services/object_manager/transformer"
	containerSDK "github.com/TrueCloudLab/frostfs-sdk-go/container"
	"github.com/TrueCloudLab/frostfs-sdk-go/object"
	"github.com/TrueCloudLab/frostfs-sdk-go/user"
)

type Streamer struct {
	*cfg

	ctx context.Context

	target transformer.ObjectTarget

	relay func(client.NodeInfo, client.MultiAddressClient) error

	maxPayloadSz uint64 // network config
}

var errNotInit = errors.New("stream not initialized")

var errInitRecall = errors.New("init recall")

func (p *Streamer) Init(prm *PutInitPrm) error {
	// initialize destination target
	if err := p.initTarget(prm); err != nil {
		return fmt.Errorf("(%T) could not initialize object target: %w", p, err)
	}

	if err := p.target.WriteHeader(prm.hdr); err != nil {
		return fmt.Errorf("(%T) could not write header to target: %w", p, err)
	}
	return nil
}

// MaxObjectSize returns maximum payload size for the streaming session.
//
// Must be called after the successful Init.
func (p *Streamer) MaxObjectSize() uint64 {
	return p.maxPayloadSz
}

func (p *Streamer) initTarget(prm *PutInitPrm) error {
	// prevent re-calling
	if p.target != nil {
		return errInitRecall
	}

	// prepare needed put parameters
	if err := p.preparePrm(prm); err != nil {
		return fmt.Errorf("(%T) could not prepare put parameters: %w", p, err)
	}

	p.maxPayloadSz = p.maxSizeSrc.MaxObjectSize()
	if p.maxPayloadSz == 0 {
		return fmt.Errorf("(%T) could not obtain max object size parameter", p)
	}

	if prm.hdr.Signature() != nil {
		p.relay = prm.relay

		// prepare untrusted-Put object target
		p.target = &validatingTarget{
			nextTarget: p.newCommonTarget(prm),
			fmt:        p.fmtValidator,

			maxPayloadSz: p.maxPayloadSz,
		}

		return nil
	}

	sToken := prm.common.SessionToken()

	// prepare trusted-Put object target

	// get private token from local storage
	var sessionInfo *util.SessionInfo

	if sToken != nil {
		sessionInfo = &util.SessionInfo{
			ID:    sToken.ID(),
			Owner: sToken.Issuer(),
		}
	}

	sessionKey, err := p.keyStorage.GetKey(sessionInfo)
	if err != nil {
		return fmt.Errorf("(%T) could not receive session key: %w", p, err)
	}

	// In case session token is missing, the line above returns the default key.
	// If it isn't owner key, replication attempts will fail, thus this check.
	if sToken == nil {
		ownerObj := prm.hdr.OwnerID()
		if ownerObj == nil {
			return errors.New("missing object owner")
		}

		var ownerSession user.ID
		user.IDFromKey(&ownerSession, sessionKey.PublicKey)

		if !ownerObj.Equals(ownerSession) {
			return fmt.Errorf("(%T) session token is missing but object owner id is different from the default key", p)
		}
	}

	p.target = &validatingTarget{
		fmt:              p.fmtValidator,
		unpreparedObject: true,
		nextTarget: transformer.NewPayloadSizeLimiter(
			p.maxPayloadSz,
			containerSDK.IsHomomorphicHashingDisabled(prm.cnr),
			func() transformer.ObjectTarget {
				return transformer.NewFormatTarget(&transformer.FormatterParams{
					Key:          sessionKey,
					NextTarget:   p.newCommonTarget(prm),
					SessionToken: sToken,
					NetworkState: p.networkState,
				})
			},
		),
	}

	return nil
}

func (p *Streamer) preparePrm(prm *PutInitPrm) error {
	var err error

	// get latest network map
	nm, err := netmap.GetLatestNetworkMap(p.netMapSrc)
	if err != nil {
		return fmt.Errorf("(%T) could not get latest network map: %w", p, err)
	}

	idCnr, ok := prm.hdr.ContainerID()
	if !ok {
		return errors.New("missing container ID")
	}

	// get container to store the object
	cnrInfo, err := p.cnrSrc.Get(idCnr)
	if err != nil {
		return fmt.Errorf("(%T) could not get container by ID: %w", p, err)
	}

	prm.cnr = cnrInfo.Value

	// add common options
	prm.traverseOpts = append(prm.traverseOpts,
		// set processing container
		placement.ForContainer(prm.cnr),
	)

	if id, ok := prm.hdr.ID(); ok {
		prm.traverseOpts = append(prm.traverseOpts,
			// set identifier of the processing object
			placement.ForObject(id),
		)
	}

	// create placement builder from network map
	builder := placement.NewNetworkMapBuilder(nm)

	if prm.common.LocalOnly() {
		// restrict success count to 1 stored copy (to local storage)
		prm.traverseOpts = append(prm.traverseOpts, placement.SuccessAfter(1))

		// use local-only placement builder
		builder = util.NewLocalPlacement(builder, p.netmapKeys)
	}

	// set placement builder
	prm.traverseOpts = append(prm.traverseOpts, placement.UseBuilder(builder))

	return nil
}

func (p *Streamer) newCommonTarget(prm *PutInitPrm) transformer.ObjectTarget {
	var relay func(nodeDesc) error
	if p.relay != nil {
		relay = func(node nodeDesc) error {
			var info client.NodeInfo

			client.NodeInfoFromNetmapElement(&info, node.info)

			c, err := p.clientConstructor.Get(info)
			if err != nil {
				return fmt.Errorf("could not create SDK client %s: %w", info.AddressGroup(), err)
			}

			return p.relay(info, c)
		}
	}

	// enable additional container broadcast on non-local operation
	// if object has TOMBSTONE or LOCK type.
	typ := prm.hdr.Type()
	withBroadcast := !prm.common.LocalOnly() && (typ == object.TypeTombstone || typ == object.TypeLock)

	return &distributedTarget{
		traversal: traversal{
			opts: prm.traverseOpts,

			extraBroadcastEnabled: withBroadcast,
		},
		payload:    getPayload(),
		remotePool: p.remotePool,
		localPool:  p.localPool,
		nodeTargetInitializer: func(node nodeDesc) preparedObjectTarget {
			if node.local {
				return &localTarget{
					storage: p.localStore,
				}
			}

			rt := &remoteTarget{
				ctx:               p.ctx,
				keyStorage:        p.keyStorage,
				commonPrm:         prm.common,
				clientConstructor: p.clientConstructor,
			}

			client.NodeInfoFromNetmapElement(&rt.nodeInfo, node.info)

			return rt
		},
		relay: relay,
		fmt:   p.fmtValidator,
		log:   p.log,

		isLocalKey: p.netmapKeys.IsLocalKey,
	}
}

func (p *Streamer) SendChunk(prm *PutChunkPrm) error {
	if p.target == nil {
		return errNotInit
	}

	if _, err := p.target.Write(prm.chunk); err != nil {
		return fmt.Errorf("(%T) could not write payload chunk to target: %w", p, err)
	}

	return nil
}

func (p *Streamer) Close() (*PutResponse, error) {
	if p.target == nil {
		return nil, errNotInit
	}

	ids, err := p.target.Close()
	if err != nil {
		return nil, fmt.Errorf("(%T) could not close object target: %w", p, err)
	}

	id := ids.ParentID()
	if id != nil {
		return &PutResponse{
			id: *id,
		}, nil
	}

	return &PutResponse{
		id: ids.SelfID(),
	}, nil
}
