package blobovnicza

import (
	common "github.com/TrueCloudLab/frostfs-node/cmd/frostfs-lens/internal"
	"github.com/TrueCloudLab/frostfs-node/pkg/local_object_storage/blobovnicza"
	"github.com/TrueCloudLab/frostfs-sdk-go/object"
	oid "github.com/TrueCloudLab/frostfs-sdk-go/object/id"
	"github.com/spf13/cobra"
)

var inspectCMD = &cobra.Command{
	Use:   "inspect",
	Short: "Object inspection",
	Long:  `Inspect specific object in a blobovnicza.`,
	Run:   inspectFunc,
}

func init() {
	common.AddAddressFlag(inspectCMD, &vAddress)
	common.AddComponentPathFlag(inspectCMD, &vPath)
	common.AddOutputFileFlag(inspectCMD, &vOut)
}

func inspectFunc(cmd *cobra.Command, _ []string) {
	var addr oid.Address

	err := addr.DecodeString(vAddress)
	common.ExitOnErr(cmd, common.Errf("invalid address argument: %w", err))

	blz := openBlobovnicza(cmd)
	defer blz.Close()

	var prm blobovnicza.GetPrm
	prm.SetAddress(addr)

	res, err := blz.Get(prm)
	common.ExitOnErr(cmd, common.Errf("could not fetch object: %w", err))

	data := res.Object()

	var o object.Object
	common.ExitOnErr(cmd, common.Errf("could not unmarshal object: %w",
		o.Unmarshal(data)),
	)

	common.PrintObjectHeader(cmd, o)
	common.WriteObjectToFile(cmd, vOut, data)
}
