package control

import (
	"crypto/sha256"
	"errors"

	rawclient "github.com/TrueCloudLab/frostfs-api-go/v2/rpc/client"
	"github.com/TrueCloudLab/frostfs-node/cmd/frostfs-cli/internal/common"
	"github.com/TrueCloudLab/frostfs-node/cmd/frostfs-cli/internal/commonflags"
	"github.com/TrueCloudLab/frostfs-node/cmd/frostfs-cli/internal/key"
	"github.com/TrueCloudLab/frostfs-node/pkg/services/control"
	controlSvc "github.com/TrueCloudLab/frostfs-node/pkg/services/control/server"
	cid "github.com/TrueCloudLab/frostfs-sdk-go/container/id"
	"github.com/spf13/cobra"
)

const (
	synchronizeTreeIDFlag     = "tree-id"
	synchronizeTreeHeightFlag = "height"
)

var synchronizeTreeCmd = &cobra.Command{
	Use:   "synchronize-tree",
	Short: "Synchronize log for the tree",
	Long:  "Synchronize log for the tree in an object tree service.",
	Run:   synchronizeTree,
}

func initControlSynchronizeTreeCmd() {
	initControlFlags(synchronizeTreeCmd)

	flags := synchronizeTreeCmd.Flags()
	flags.String(commonflags.CIDFlag, "", commonflags.CIDFlagUsage)
	flags.String(synchronizeTreeIDFlag, "", "Tree ID")
	flags.Uint64(synchronizeTreeHeightFlag, 0, "Starting height")
}

func synchronizeTree(cmd *cobra.Command, _ []string) {
	pk := key.Get(cmd)

	var cnr cid.ID
	cidStr, _ := cmd.Flags().GetString(commonflags.CIDFlag)
	common.ExitOnErr(cmd, "can't decode container ID: %w", cnr.DecodeString(cidStr))

	treeID, _ := cmd.Flags().GetString("tree-id")
	if treeID == "" {
		common.ExitOnErr(cmd, "", errors.New("tree ID must not be empty"))
	}

	height, _ := cmd.Flags().GetUint64("height")

	rawCID := make([]byte, sha256.Size)
	cnr.Encode(rawCID)

	req := &control.SynchronizeTreeRequest{
		Body: &control.SynchronizeTreeRequest_Body{
			ContainerId: rawCID,
			TreeId:      treeID,
			Height:      height,
		},
	}

	err := controlSvc.SignMessage(pk, req)
	common.ExitOnErr(cmd, "could not sign request: %w", err)

	cli := getClient(cmd, pk)

	var resp *control.SynchronizeTreeResponse
	err = cli.ExecRaw(func(client *rawclient.Client) error {
		resp, err = control.SynchronizeTree(client, req)
		return err
	})
	common.ExitOnErr(cmd, "rpc error: %w", err)

	verifyResponse(cmd, resp.GetSignature(), resp.GetBody())

	cmd.Println("Tree has been synchronized successfully.")
}
