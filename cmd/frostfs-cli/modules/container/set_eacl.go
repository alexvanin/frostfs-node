package container

import (
	"bytes"
	"errors"
	"time"

	internalclient "github.com/TrueCloudLab/frostfs-node/cmd/frostfs-cli/internal/client"
	"github.com/TrueCloudLab/frostfs-node/cmd/frostfs-cli/internal/common"
	"github.com/TrueCloudLab/frostfs-node/cmd/frostfs-cli/internal/commonflags"
	"github.com/TrueCloudLab/frostfs-node/cmd/frostfs-cli/internal/key"
	"github.com/spf13/cobra"
)

var flagVarsSetEACL struct {
	noPreCheck bool

	srcPath string
}

var setExtendedACLCmd = &cobra.Command{
	Use:   "set-eacl",
	Short: "Set new extended ACL table for container",
	Long: `Set new extended ACL table for container.
Container ID in EACL table will be substituted with ID from the CLI.`,
	Run: func(cmd *cobra.Command, args []string) {
		id := parseContainerID(cmd)
		eaclTable := common.ReadEACL(cmd, flagVarsSetEACL.srcPath)

		tok := getSession(cmd)

		eaclTable.SetCID(id)

		pk := key.GetOrGenerate(cmd)
		cli := internalclient.GetSDKClientByFlag(cmd, pk, commonflags.RPC)

		if !flagVarsSetEACL.noPreCheck {
			cmd.Println("Checking the ability to modify access rights in the container...")

			extendable, err := internalclient.IsACLExtendable(cli, id)
			common.ExitOnErr(cmd, "Extensibility check failure: %w", err)

			if !extendable {
				common.ExitOnErr(cmd, "", errors.New("container ACL is immutable"))
			}

			cmd.Println("ACL extension is enabled in the container, continue processing.")
		}

		var setEACLPrm internalclient.SetEACLPrm
		setEACLPrm.SetClient(cli)
		setEACLPrm.SetTable(*eaclTable)

		if tok != nil {
			setEACLPrm.WithinSession(*tok)
		}

		_, err := internalclient.SetEACL(setEACLPrm)
		common.ExitOnErr(cmd, "rpc error: %w", err)

		if containerAwait {
			exp, err := eaclTable.Marshal()
			common.ExitOnErr(cmd, "broken EACL table: %w", err)

			cmd.Println("awaiting...")

			var getEACLPrm internalclient.EACLPrm
			getEACLPrm.SetClient(cli)
			getEACLPrm.SetContainer(id)

			for i := 0; i < awaitTimeout; i++ {
				time.Sleep(1 * time.Second)

				res, err := internalclient.EACL(getEACLPrm)
				if err == nil {
					// compare binary values because EACL could have been set already
					table := res.EACL()
					got, err := table.Marshal()
					if err != nil {
						continue
					}

					if bytes.Equal(exp, got) {
						cmd.Println("EACL has been persisted on sidechain")
						return
					}
				}
			}

			common.ExitOnErr(cmd, "", errSetEACLTimeout)
		}
	},
}

func initContainerSetEACLCmd() {
	commonflags.Init(setExtendedACLCmd)

	flags := setExtendedACLCmd.Flags()
	flags.StringVar(&containerID, commonflags.CIDFlag, "", commonflags.CIDFlagUsage)
	flags.StringVar(&flagVarsSetEACL.srcPath, "table", "", "path to file with JSON or binary encoded EACL table")
	flags.BoolVar(&containerAwait, "await", false, "block execution until EACL is persisted")
	flags.BoolVar(&flagVarsSetEACL.noPreCheck, "no-precheck", false, "do not pre-check the extensibility of the container ACL")
}
