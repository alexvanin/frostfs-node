package morph

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/services/rpcsrv/params"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	contractPathFlag = "contract"
	updateFlag       = "update"
	customZoneFlag   = "domain"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy additional smart-contracts",
	Long: `Deploy additional smart-contract which are not related to core.
All contracts are deployed by the committee, so access to the alphabet wallets is required.
Optionally, arguments can be provided to be passed to a contract's _deploy function.
The syntax is the same as for 'neo-go contract testinvokefunction' command.`,
	PreRun: func(cmd *cobra.Command, _ []string) {
		_ = viper.BindPFlag(alphabetWalletsFlag, cmd.Flags().Lookup(alphabetWalletsFlag))
		_ = viper.BindPFlag(endpointFlag, cmd.Flags().Lookup(endpointFlag))
	},
	RunE: deployContractCmd,
}

func init() {
	ff := deployCmd.Flags()

	ff.String(alphabetWalletsFlag, "", "Path to alphabet wallets dir")
	_ = deployCmd.MarkFlagFilename(alphabetWalletsFlag)

	ff.StringP(endpointFlag, "r", "", "N3 RPC node endpoint")
	ff.String(contractPathFlag, "", "Path to the contract directory")
	_ = deployCmd.MarkFlagFilename(contractPathFlag)

	ff.Bool(updateFlag, false, "Update an existing contract")
	ff.String(customZoneFlag, "neofs", "Custom zone for NNS (default: 'neofs')")
}

func deployContractCmd(cmd *cobra.Command, args []string) error {
	v := viper.GetViper()
	c, err := newInitializeContext(cmd, v)
	if err != nil {
		return fmt.Errorf("initialization error: %w", err)
	}
	defer c.close()

	ctrPath, _ := cmd.Flags().GetString(contractPathFlag)
	ctrName, err := probeContractName(ctrPath)
	if err != nil {
		return err
	}

	cs, err := readContract(ctrPath, ctrName)
	if err != nil {
		return err
	}

	nnsCs, err := c.Client.GetContractStateByID(1)
	if err != nil {
		return fmt.Errorf("can't fetch NNS contract state: %w", err)
	}

	callHash := c.nativeHash(nativenames.Management)
	method := deployMethodName
	zone, _ := cmd.Flags().GetString(customZoneFlag)
	domain := ctrName + "." + zone
	isUpdate, _ := cmd.Flags().GetBool(updateFlag)
	if isUpdate {
		cs.Hash, err = nnsResolveHash(c.Client, nnsCs.Hash, domain)
		if err != nil {
			return fmt.Errorf("can't fetch contract hash from NNS: %w", err)
		}
		callHash = cs.Hash
		method = updateMethodName
	} else {
		cs.Hash = state.CreateContractHash(
			c.CommitteeAcc.Contract.ScriptHash(),
			cs.NEF.Checksum,
			cs.Manifest.Name)
	}

	w := io.NewBufBinWriter()
	if err := emitDeploymentArguments(w.BinWriter, args); err != nil {
		return err
	}
	emit.Bytes(w.BinWriter, cs.RawManifest)
	emit.Bytes(w.BinWriter, cs.RawNEF)
	emit.Int(w.BinWriter, 3)
	emit.Opcodes(w.BinWriter, opcode.PACK)
	emit.AppCallNoArgs(w.BinWriter, callHash, method, callflag.All)
	emit.Opcodes(w.BinWriter, opcode.DROP) // contract state on stack
	if !isUpdate {
		s, err := c.nnsRegisterDomainScript(nnsCs.Hash, cs.Hash, domain)
		if err != nil {
			return err
		}
		if len(s) != 0 {
			c.Command.Printf("NNS: Set %s -> %s\n", domain, cs.Hash.StringLE())
			w.WriteBytes(s)
		}
	}

	if w.Err != nil {
		panic(fmt.Errorf("BUG: can't create deployment script: %w", w.Err))
	}

	if err := c.sendCommitteeTx(w.Bytes(), -1, false); err != nil {
		return err
	}
	return c.awaitTx()
}

func emitDeploymentArguments(w *io.BinWriter, args []string) error {
	_, ps, err := cmdargs.ParseParams(args, true)
	if err != nil {
		return err
	}

	if len(ps) == 0 {
		emit.Opcodes(w, opcode.NEWARRAY0)
		return nil
	}

	if len(ps) != 1 {
		return fmt.Errorf("at most one argument is expected for deploy, got %d", len(ps))
	}

	// We could emit this directly, but round-trip through JSON is more robust.
	// This a CLI, so optimizing the conversion is not worth the effort.
	data, err := json.Marshal(ps)
	if err != nil {
		return err
	}

	var pp params.Params
	if err := json.Unmarshal(data, &pp); err != nil {
		return err
	}
	return params.ExpandArrayIntoScript(w, pp)
}

func probeContractName(ctrPath string) (string, error) {
	ds, err := os.ReadDir(ctrPath)
	if err != nil {
		return "", fmt.Errorf("can't read directory: %w", err)
	}

	var ctrName string
	for i := range ds {
		if strings.HasSuffix(ds[i].Name(), "_contract.nef") {
			ctrName = strings.TrimSuffix(ds[i].Name(), "_contract.nef")
			break
		}
	}

	if ctrName == "" {
		return "", fmt.Errorf("can't find any NEF files in %s", ctrPath)
	}
	return ctrName, nil
}
