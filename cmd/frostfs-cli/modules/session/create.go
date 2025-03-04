package session

import (
	"fmt"
	"os"

	internalclient "github.com/TrueCloudLab/frostfs-node/cmd/frostfs-cli/internal/client"
	"github.com/TrueCloudLab/frostfs-node/cmd/frostfs-cli/internal/common"
	"github.com/TrueCloudLab/frostfs-node/cmd/frostfs-cli/internal/commonflags"
	"github.com/TrueCloudLab/frostfs-node/cmd/frostfs-cli/internal/key"
	"github.com/TrueCloudLab/frostfs-node/pkg/network"
	"github.com/TrueCloudLab/frostfs-sdk-go/client"
	frostfsecdsa "github.com/TrueCloudLab/frostfs-sdk-go/crypto/ecdsa"
	"github.com/TrueCloudLab/frostfs-sdk-go/session"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	outFlag  = "out"
	jsonFlag = commonflags.JSON
)

const defaultLifetime = 10

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create session token",
	Run:   createSession,
	PersistentPreRun: func(cmd *cobra.Command, _ []string) {
		_ = viper.BindPFlag(commonflags.WalletPath, cmd.Flags().Lookup(commonflags.WalletPath))
		_ = viper.BindPFlag(commonflags.Account, cmd.Flags().Lookup(commonflags.Account))
	},
}

func init() {
	createCmd.Flags().Uint64P(commonflags.Lifetime, "l", defaultLifetime, "Number of epochs for token to stay valid")
	createCmd.Flags().StringP(commonflags.WalletPath, commonflags.WalletPathShorthand, commonflags.WalletPathDefault, commonflags.WalletPathUsage)
	createCmd.Flags().StringP(commonflags.Account, commonflags.AccountShorthand, commonflags.AccountDefault, commonflags.AccountUsage)
	createCmd.Flags().String(outFlag, "", "File to write session token to")
	createCmd.Flags().Bool(jsonFlag, false, "Output token in JSON")
	createCmd.Flags().StringP(commonflags.RPC, commonflags.RPCShorthand, commonflags.RPCDefault, commonflags.RPCUsage)

	_ = cobra.MarkFlagRequired(createCmd.Flags(), commonflags.WalletPath)
	_ = cobra.MarkFlagRequired(createCmd.Flags(), outFlag)
	_ = cobra.MarkFlagRequired(createCmd.Flags(), commonflags.RPC)
}

func createSession(cmd *cobra.Command, _ []string) {
	privKey := key.Get(cmd)

	var netAddr network.Address
	addrStr, _ := cmd.Flags().GetString(commonflags.RPC)
	common.ExitOnErr(cmd, "can't parse endpoint: %w", netAddr.FromString(addrStr))

	c, err := internalclient.GetSDKClient(cmd, privKey, netAddr)
	common.ExitOnErr(cmd, "can't create client: %w", err)

	lifetime := uint64(defaultLifetime)
	if lfArg, _ := cmd.Flags().GetUint64(commonflags.Lifetime); lfArg != 0 {
		lifetime = lfArg
	}

	var tok session.Object

	err = CreateSession(&tok, c, lifetime)
	common.ExitOnErr(cmd, "can't create session: %w", err)

	var data []byte

	if toJSON, _ := cmd.Flags().GetBool(jsonFlag); toJSON {
		data, err = tok.MarshalJSON()
		common.ExitOnErr(cmd, "can't decode session token JSON: %w", err)
	} else {
		data = tok.Marshal()
	}

	filename, _ := cmd.Flags().GetString(outFlag)
	err = os.WriteFile(filename, data, 0644)
	common.ExitOnErr(cmd, "can't write token to file: %w", err)
}

// CreateSession opens a new communication with NeoFS storage node using client connection.
// The session is expected to be maintained by the storage node during the given
// number of epochs.
//
// Fills ID, lifetime and session key.
func CreateSession(dst *session.Object, c *client.Client, lifetime uint64) error {
	var netInfoPrm internalclient.NetworkInfoPrm
	netInfoPrm.SetClient(c)

	ni, err := internalclient.NetworkInfo(netInfoPrm)
	if err != nil {
		return fmt.Errorf("can't fetch network info: %w", err)
	}

	cur := ni.NetworkInfo().CurrentEpoch()
	exp := cur + lifetime

	var sessionPrm internalclient.CreateSessionPrm
	sessionPrm.SetClient(c)
	sessionPrm.SetExp(exp)

	sessionRes, err := internalclient.CreateSession(sessionPrm)
	if err != nil {
		return fmt.Errorf("can't open session: %w", err)
	}

	binIDSession := sessionRes.ID()

	var keySession frostfsecdsa.PublicKey

	err = keySession.Decode(sessionRes.SessionKey())
	if err != nil {
		return fmt.Errorf("decode public session key: %w", err)
	}

	var idSession uuid.UUID

	err = idSession.UnmarshalBinary(binIDSession)
	if err != nil {
		return fmt.Errorf("decode session ID: %w", err)
	}

	dst.SetID(idSession)
	dst.SetNbf(cur)
	dst.SetIat(cur)
	dst.SetExp(exp)
	dst.SetAuthKey(&keySession)

	return nil
}
