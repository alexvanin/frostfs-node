package netmap

import (
	v2netmap "github.com/TrueCloudLab/frostfs-api-go/v2/netmap"
	netmapclient "github.com/TrueCloudLab/frostfs-node/pkg/morph/client/netmap"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"go.uber.org/zap"
)

func (np *Processor) processNetmapCleanupTick(ev netmapCleanupTick) {
	if !np.alphabetState.IsAlphabet() {
		np.log.Info("non alphabet mode, ignore new netmap cleanup tick")

		return
	}

	err := np.netmapSnapshot.forEachRemoveCandidate(ev.epoch, func(s string) error {
		key, err := keys.NewPublicKeyFromString(s)
		if err != nil {
			np.log.Warn("can't decode public key of netmap node",
				zap.String("key", s))

			return nil
		}

		np.log.Info("vote to remove node from netmap", zap.String("key", s))

		// In notary environments we call UpdateStateIR method instead of UpdateState.
		// It differs from UpdateState only by name, so we can do this in the same form.
		// See https://github.com/nspcc-dev/frostfs-contract/issues/225
		const methodUpdateStateNotary = "updateStateIR"

		if np.notaryDisabled {
			prm := netmapclient.UpdatePeerPrm{}

			prm.SetKey(key.Bytes())
			prm.SetHash(ev.TxHash())

			err = np.netmapClient.UpdatePeerState(prm)
		} else {
			err = np.netmapClient.Morph().NotaryInvoke(
				np.netmapClient.ContractAddress(),
				0,
				uint32(ev.epoch),
				nil,
				methodUpdateStateNotary,
				int64(v2netmap.Offline), key.Bytes(),
			)
		}
		if err != nil {
			np.log.Error("can't invoke netmap.UpdateState", zap.Error(err))
		}

		return nil
	})
	if err != nil {
		np.log.Warn("can't iterate on netmap cleaner cache",
			zap.String("error", err.Error()))
	}
}
