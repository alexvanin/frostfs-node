package putsvc

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"

	"github.com/TrueCloudLab/frostfs-node/pkg/core/object"
	"github.com/TrueCloudLab/frostfs-node/pkg/services/object_manager/transformer"
	"github.com/TrueCloudLab/frostfs-sdk-go/checksum"
	objectSDK "github.com/TrueCloudLab/frostfs-sdk-go/object"
	"github.com/TrueCloudLab/tzhash/tz"
)

// validatingTarget validates object format and content.
type validatingTarget struct {
	nextTarget transformer.ObjectTarget

	fmt *object.FormatValidator

	unpreparedObject bool

	hash hash.Hash

	checksum []byte

	maxPayloadSz uint64 // network config

	payloadSz uint64 // payload size of the streaming object from header

	writtenPayload uint64 // number of already written payload bytes
}

var (
	// ErrExceedingMaxSize is returned when payload size is greater than the limit.
	ErrExceedingMaxSize = errors.New("payload size is greater than the limit")
	// ErrWrongPayloadSize is returned when chunk payload size is greater than the length declared in header.
	ErrWrongPayloadSize = errors.New("wrong payload size")
)

func (t *validatingTarget) WriteHeader(obj *objectSDK.Object) error {
	t.payloadSz = obj.PayloadSize()
	chunkLn := uint64(len(obj.Payload()))

	if !t.unpreparedObject {
		// check chunk size
		if chunkLn > t.payloadSz {
			return ErrWrongPayloadSize
		}

		// check payload size limit
		if t.payloadSz > t.maxPayloadSz {
			return ErrExceedingMaxSize
		}

		cs, csSet := obj.PayloadChecksum()
		if !csSet {
			return errors.New("missing payload checksum")
		}

		switch typ := cs.Type(); typ {
		default:
			return fmt.Errorf("(%T) unsupported payload checksum type %v", t, typ)
		case checksum.SHA256:
			t.hash = sha256.New()
		case checksum.TZ:
			t.hash = tz.New()
		}

		t.checksum = cs.Value()
	}

	if err := t.fmt.Validate(obj, t.unpreparedObject); err != nil {
		return fmt.Errorf("(%T) coult not validate object format: %w", t, err)
	}

	err := t.nextTarget.WriteHeader(obj)
	if err != nil {
		return err
	}

	if !t.unpreparedObject {
		// update written bytes
		//
		// Note: we MUST NOT add obj.PayloadSize() since obj
		// can carry only the chunk of the full payload
		t.writtenPayload += chunkLn
	}

	return nil
}

func (t *validatingTarget) Write(p []byte) (n int, err error) {
	chunkLn := uint64(len(p))

	if !t.unpreparedObject {
		// check if new chunk will overflow payload size
		if t.writtenPayload+chunkLn > t.payloadSz {
			return 0, ErrWrongPayloadSize
		}

		_, err = t.hash.Write(p)
		if err != nil {
			return
		}
	}

	n, err = t.nextTarget.Write(p)
	if err == nil {
		t.writtenPayload += uint64(n)
	}

	return
}

func (t *validatingTarget) Close() (*transformer.AccessIdentifiers, error) {
	if !t.unpreparedObject {
		// check payload size correctness
		if t.payloadSz != t.writtenPayload {
			return nil, ErrWrongPayloadSize
		}

		if !bytes.Equal(t.hash.Sum(nil), t.checksum) {
			return nil, fmt.Errorf("(%T) incorrect payload checksum", t)
		}
	}

	return t.nextTarget.Close()
}
