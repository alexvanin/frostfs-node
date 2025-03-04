package netmap

import (
	"encoding/hex"

	internalclient "github.com/TrueCloudLab/frostfs-node/cmd/frostfs-cli/internal/client"
	"github.com/TrueCloudLab/frostfs-node/cmd/frostfs-cli/internal/common"
	"github.com/TrueCloudLab/frostfs-node/cmd/frostfs-cli/internal/commonflags"
	"github.com/TrueCloudLab/frostfs-node/cmd/frostfs-cli/internal/key"
	"github.com/TrueCloudLab/frostfs-sdk-go/netmap"
	"github.com/spf13/cobra"
)

const nodeInfoJSONFlag = commonflags.JSON

var nodeInfoCmd = &cobra.Command{
	Use:   "nodeinfo",
	Short: "Get target node info",
	Long:  `Get target node info`,
	Run: func(cmd *cobra.Command, args []string) {
		p := key.GetOrGenerate(cmd)
		cli := internalclient.GetSDKClientByFlag(cmd, p, commonflags.RPC)

		var prm internalclient.NodeInfoPrm
		prm.SetClient(cli)

		res, err := internalclient.NodeInfo(prm)
		common.ExitOnErr(cmd, "rpc error: %w", err)

		prettyPrintNodeInfo(cmd, res.NodeInfo())
	},
}

func initNodeInfoCmd() {
	commonflags.Init(nodeInfoCmd)
	commonflags.InitAPI(nodeInfoCmd)
	nodeInfoCmd.Flags().Bool(nodeInfoJSONFlag, false, "Print node info in JSON format")
}

func prettyPrintNodeInfo(cmd *cobra.Command, i netmap.NodeInfo) {
	isJSON, _ := cmd.Flags().GetBool(nodeInfoJSONFlag)
	if isJSON {
		common.PrettyPrintJSON(cmd, i, "node info")
		return
	}

	cmd.Println("key:", hex.EncodeToString(i.PublicKey()))

	var stateWord string
	switch {
	default:
		stateWord = "<undefined>"
	case i.IsOnline():
		stateWord = "online"
	case i.IsOffline():
		stateWord = "offline"
	case i.IsMaintenance():
		stateWord = "maintenance"
	}

	cmd.Println("state:", stateWord)

	netmap.IterateNetworkEndpoints(i, func(s string) {
		cmd.Println("address:", s)
	})

	i.IterateAttributes(func(key, value string) {
		cmd.Printf("attribute: %s=%s\n", key, value)
	})
}
