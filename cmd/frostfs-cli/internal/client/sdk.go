package internal

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/TrueCloudLab/frostfs-node/cmd/frostfs-cli/internal/common"
	"github.com/TrueCloudLab/frostfs-node/cmd/frostfs-cli/internal/commonflags"
	"github.com/TrueCloudLab/frostfs-node/pkg/network"
	"github.com/TrueCloudLab/frostfs-sdk-go/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var errInvalidEndpoint = errors.New("provided RPC endpoint is incorrect")

// GetSDKClientByFlag returns default frostfs-sdk-go client using the specified flag for the address.
// On error, outputs to stderr of cmd and exits with non-zero code.
func GetSDKClientByFlag(cmd *cobra.Command, key *ecdsa.PrivateKey, endpointFlag string) *client.Client {
	cli, err := getSDKClientByFlag(cmd, key, endpointFlag)
	if err != nil {
		common.ExitOnErr(cmd, "can't create API client: %w", err)
	}
	return cli
}

func getSDKClientByFlag(cmd *cobra.Command, key *ecdsa.PrivateKey, endpointFlag string) (*client.Client, error) {
	var addr network.Address

	err := addr.FromString(viper.GetString(endpointFlag))
	if err != nil {
		return nil, fmt.Errorf("%v: %w", errInvalidEndpoint, err)
	}
	return GetSDKClient(cmd, key, addr)
}

// GetSDKClient returns default frostfs-sdk-go client.
func GetSDKClient(cmd *cobra.Command, key *ecdsa.PrivateKey, addr network.Address) (*client.Client, error) {
	var (
		c       client.Client
		prmInit client.PrmInit
		prmDial client.PrmDial
	)

	prmInit.SetDefaultPrivateKey(*key)
	prmInit.ResolveNeoFSFailures()
	prmDial.SetServerURI(addr.URIAddr())
	if timeout := viper.GetDuration(commonflags.Timeout); timeout > 0 {
		// In CLI we can only set a timeout for the whole operation.
		// By also setting stream timeout we ensure that no operation hands
		// for too long.
		prmDial.SetTimeout(timeout)
		prmDial.SetStreamTimeout(timeout)

		common.PrintVerbose(cmd, "Set request timeout to %s.", timeout)
	}

	c.Init(prmInit)

	if err := c.Dial(prmDial); err != nil {
		return nil, fmt.Errorf("can't init SDK client: %w", err)
	}

	return &c, nil
}

// GetCurrentEpoch returns current epoch.
func GetCurrentEpoch(ctx context.Context, cmd *cobra.Command, endpoint string) (uint64, error) {
	var addr network.Address

	if err := addr.FromString(endpoint); err != nil {
		return 0, fmt.Errorf("can't parse RPC endpoint: %w", err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return 0, fmt.Errorf("can't generate key to sign query: %w", err)
	}

	c, err := GetSDKClient(cmd, key, addr)
	if err != nil {
		return 0, err
	}

	ni, err := c.NetworkInfo(ctx, client.PrmNetworkInfo{})
	if err != nil {
		return 0, err
	}

	return ni.Info().CurrentEpoch(), nil
}
