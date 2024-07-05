package vf3

import (
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/filecoin-project/go-f3/gpbft"
	"github.com/filecoin-project/go-f3/manifest"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/statemanger"
)

func NewManifestProvider(nn string, cs *chain.Store, sm *statemanger.Stmgr, ps *pubsub.PubSub, networkParams *config.NetworkParamsConfig) manifest.ManifestProvider {
	m := manifest.LocalDevnetManifest()
	m.NetworkName = gpbft.NetworkName(nn)
	m.ECDelay = 2 * time.Duration(networkParams.BlockDelay) * time.Second
	m.ECPeriod = m.ECDelay
	m.BootstrapEpoch = int64(networkParams.F3BootstrapEpoch)
	m.ECFinality = int64(constants.Finality)
	m.CommiteeLookback = 5

	ec := &ecWrapper{
		ChainStore:   cs,
		StateManager: sm,
	}

	switch manifestServerID, err := peer.Decode(networkParams.ManifestServerID); {
	case err != nil:
		log.Warnw("Cannot decode F3 manifest sever identity; falling back on static manifest provider", "err", err)
		return manifest.NewStaticManifestProvider(m)
	default:
		return manifest.NewDynamicManifestProvider(m, ps, ec, manifestServerID)
	}
}
