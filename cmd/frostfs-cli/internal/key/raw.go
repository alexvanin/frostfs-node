package key

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"os"

	"github.com/TrueCloudLab/frostfs-node/cmd/frostfs-cli/internal/common"
	"github.com/TrueCloudLab/frostfs-node/cmd/frostfs-cli/internal/commonflags"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var errCantGenerateKey = errors.New("can't generate new private key")

// Get returns private key from wallet or binary file.
// Ideally we want to touch file-system on the last step.
// This function assumes that all flags were bind to viper in a `PersistentPreRun`.
func Get(cmd *cobra.Command) *ecdsa.PrivateKey {
	pk, err := get(cmd)
	common.ExitOnErr(cmd, "can't fetch private key: %w", err)
	return pk
}

func get(cmd *cobra.Command) (*ecdsa.PrivateKey, error) {
	keyDesc := viper.GetString(commonflags.WalletPath)
	data, err := os.ReadFile(keyDesc)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFs, err)
	}

	priv, err := keys.NewPrivateKeyFromBytes(data)
	if err != nil {
		w, err := wallet.NewWalletFromFile(keyDesc)
		if err == nil {
			return FromWallet(cmd, w, viper.GetString(commonflags.Account))
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidKey, err)
	}
	return &priv.PrivateKey, nil
}

// GetOrGenerate is similar to get but generates a new key if commonflags.GenerateKey is set.
func GetOrGenerate(cmd *cobra.Command) *ecdsa.PrivateKey {
	pk, err := getOrGenerate(cmd)
	common.ExitOnErr(cmd, "can't fetch private key: %w", err)
	return pk
}

func getOrGenerate(cmd *cobra.Command) (*ecdsa.PrivateKey, error) {
	if viper.GetBool(commonflags.GenerateKey) {
		priv, err := keys.NewPrivateKey()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errCantGenerateKey, err)
		}
		return &priv.PrivateKey, nil
	}
	return get(cmd)
}
