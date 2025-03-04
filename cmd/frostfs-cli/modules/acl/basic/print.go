package basic

import (
	"github.com/TrueCloudLab/frostfs-node/cmd/frostfs-cli/internal/common"
	"github.com/TrueCloudLab/frostfs-node/cmd/frostfs-cli/modules/util"
	"github.com/TrueCloudLab/frostfs-sdk-go/container/acl"
	"github.com/spf13/cobra"
)

var printACLCmd = &cobra.Command{
	Use:     "print",
	Short:   "Pretty print basic ACL from the HEX representation",
	Example: `frostfs-cli acl basic print 0x1C8C8CCC`,
	Long: `Pretty print basic ACL from the HEX representation.
Few roles have exclusive default access to set of operation, even if particular bit deny it.
Container have access to the operations of the data replication mechanism:
    Get, Head, Put, Search, Hash.
InnerRing members are allowed to data audit ops only:
    Get, Head, Hash, Search.`,
	Run:  printACL,
	Args: cobra.ExactArgs(1),
}

func printACL(cmd *cobra.Command, args []string) {
	var bacl acl.Basic
	common.ExitOnErr(cmd, "unable to parse basic acl: %w", bacl.DecodeString(args[0]))
	util.PrettyPrintTableBACL(cmd, &bacl)
}
