package container

import (
	"fmt"

	"github.com/TrueCloudLab/frostfs-node/pkg/morph/client"
	"github.com/TrueCloudLab/frostfs-node/pkg/morph/event"
	cid "github.com/TrueCloudLab/frostfs-sdk-go/container/id"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
)

// Put structure of container.Put notification from morph chain.
type Put struct {
	rawContainer []byte
	signature    []byte
	publicKey    []byte
	token        []byte

	// For notary notifications only.
	// Contains raw transactions of notary request.
	notaryRequest *payload.P2PNotaryRequest
}

const expectedItemNumPut = 4

// MorphEvent implements Neo:Morph Event interface.
func (Put) MorphEvent() {}

// Container is a marshalled container structure, defined in API.
func (p Put) Container() []byte { return p.rawContainer }

// Signature of marshalled container by container owner.
func (p Put) Signature() []byte { return p.signature }

// PublicKey of container owner.
func (p Put) PublicKey() []byte { return p.publicKey }

// SessionToken returns binary token of the session
// within which the container was created.
func (p Put) SessionToken() []byte {
	return p.token
}

// NotaryRequest returns raw notary request if notification
// was received via notary service. Otherwise, returns nil.
func (p Put) NotaryRequest() *payload.P2PNotaryRequest {
	return p.notaryRequest
}

// PutNamed represents notification event spawned by PutNamed method from Container contract of NeoFS Morph chain.
type PutNamed struct {
	Put

	name, zone string
}

// Name returns "name" arg of contract call.
func (x PutNamed) Name() string {
	return x.name
}

// Zone returns "zone" arg of contract call.
func (x PutNamed) Zone() string {
	return x.zone
}

// ParsePut from notification into container event structure.
func ParsePut(e *state.ContainedNotificationEvent) (event.Event, error) {
	var (
		ev  Put
		err error
	)

	params, err := event.ParseStackArray(e)
	if err != nil {
		return nil, fmt.Errorf("could not parse stack items from notify event: %w", err)
	}

	if ln := len(params); ln != expectedItemNumPut {
		return nil, event.WrongNumberOfParameters(expectedItemNumPut, ln)
	}

	// parse container
	ev.rawContainer, err = client.BytesFromStackItem(params[0])
	if err != nil {
		return nil, fmt.Errorf("could not get container: %w", err)
	}

	// parse signature
	ev.signature, err = client.BytesFromStackItem(params[1])
	if err != nil {
		return nil, fmt.Errorf("could not get signature: %w", err)
	}

	// parse public key
	ev.publicKey, err = client.BytesFromStackItem(params[2])
	if err != nil {
		return nil, fmt.Errorf("could not get public key: %w", err)
	}

	// parse session token
	ev.token, err = client.BytesFromStackItem(params[3])
	if err != nil {
		return nil, fmt.Errorf("could not get sesison token: %w", err)
	}

	return ev, nil
}

// PutSuccess structures notification event of successful container creation
// thrown by Container contract.
type PutSuccess struct {
	// Identifier of the newly created container.
	ID cid.ID
}

// MorphEvent implements Neo:Morph Event interface.
func (PutSuccess) MorphEvent() {}

// ParsePutSuccess decodes notification event thrown by Container contract into
// PutSuccess and returns it as event.Event.
func ParsePutSuccess(e *state.ContainedNotificationEvent) (event.Event, error) {
	items, err := event.ParseStackArray(e)
	if err != nil {
		return nil, fmt.Errorf("parse stack array from raw notification event: %w", err)
	}

	const expectedItemNumPutSuccess = 2

	if ln := len(items); ln != expectedItemNumPutSuccess {
		return nil, event.WrongNumberOfParameters(expectedItemNumPutSuccess, ln)
	}

	binID, err := client.BytesFromStackItem(items[0])
	if err != nil {
		return nil, fmt.Errorf("parse container ID item: %w", err)
	}

	_, err = client.BytesFromStackItem(items[1])
	if err != nil {
		return nil, fmt.Errorf("parse public key item: %w", err)
	}

	var res PutSuccess

	err = res.ID.Decode(binID)
	if err != nil {
		return nil, fmt.Errorf("decode container ID: %w", err)
	}

	return res, nil
}
