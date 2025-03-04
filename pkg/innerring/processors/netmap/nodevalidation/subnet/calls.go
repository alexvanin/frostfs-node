package subnet

import (
	"fmt"

	morphsubnet "github.com/TrueCloudLab/frostfs-node/pkg/morph/client/subnet"
	"github.com/TrueCloudLab/frostfs-sdk-go/netmap"
	subnetid "github.com/TrueCloudLab/frostfs-sdk-go/subnet/id"
)

// VerifyAndUpdate calls subnet contract's `NodeAllowed` method.
// Removes subnets that have not been approved by the contract.
func (v *Validator) VerifyAndUpdate(n *netmap.NodeInfo) error {
	prm := morphsubnet.NodeAllowedPrm{}

	err := n.IterateSubnets(func(id subnetid.ID) error {
		// every node can be bootstrapped
		// to the zero subnetwork
		if subnetid.IsZero(id) {
			return nil
		}

		prm.SetID(id.Marshal())
		prm.SetNode(n.PublicKey())

		res, err := v.subnetClient.NodeAllowed(prm)
		if err != nil {
			return fmt.Errorf("could not call `NodeAllowed` contract method: %w", err)
		}

		if !res.Allowed() {
			return netmap.ErrRemoveSubnet
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("could not verify subnet entrance of the node: %w", err)
	}

	return nil
}
