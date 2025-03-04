package container

import (
	"fmt"

	"github.com/TrueCloudLab/frostfs-api-go/v2/refs"
	containercore "github.com/TrueCloudLab/frostfs-node/pkg/core/container"
	"github.com/TrueCloudLab/frostfs-node/pkg/morph/client"
)

// PutEACL marshals table, and passes it to Wrapper's PutEACLBinary method
// along with sig.Key() and sig.Sign().
//
// Returns error if table is nil.
//
// If TryNotary is provided, calls notary contract.
func PutEACL(c *Client, eaclInfo containercore.EACL) error {
	if eaclInfo.Value == nil {
		return errNilArgument
	}

	data, err := eaclInfo.Value.Marshal()
	if err != nil {
		return fmt.Errorf("can't marshal eacl table: %w", err)
	}

	var prm PutEACLPrm
	prm.SetTable(data)

	if eaclInfo.Session != nil {
		prm.SetToken(eaclInfo.Session.Marshal())
	}

	// TODO(@cthulhu-rider): #1387 implement and use another approach to avoid conversion
	var sigV2 refs.Signature
	eaclInfo.Signature.WriteToV2(&sigV2)

	prm.SetKey(sigV2.GetKey())
	prm.SetSignature(sigV2.GetSign())

	return c.PutEACL(prm)
}

// PutEACLPrm groups parameters of PutEACL operation.
type PutEACLPrm struct {
	table []byte
	key   []byte
	sig   []byte
	token []byte

	client.InvokePrmOptional
}

// SetTable sets table.
func (p *PutEACLPrm) SetTable(table []byte) {
	p.table = table
}

// SetKey sets key.
func (p *PutEACLPrm) SetKey(key []byte) {
	p.key = key
}

// SetSignature sets signature.
func (p *PutEACLPrm) SetSignature(sig []byte) {
	p.sig = sig
}

// SetToken sets session token.
func (p *PutEACLPrm) SetToken(token []byte) {
	p.token = token
}

// PutEACL saves binary eACL table with its session token, key and signature
// in NeoFS system through Container contract call.
//
// Returns any error encountered that caused the saving to interrupt.
func (c *Client) PutEACL(p PutEACLPrm) error {
	if len(p.sig) == 0 || len(p.key) == 0 {
		return errNilArgument
	}

	prm := client.InvokePrm{}
	prm.SetMethod(setEACLMethod)
	prm.SetArgs(p.table, p.sig, p.key, p.token)
	prm.InvokePrmOptional = p.InvokePrmOptional

	err := c.client.Invoke(prm)
	if err != nil {
		return fmt.Errorf("could not invoke method (%s): %w", setEACLMethod, err)
	}
	return nil
}
