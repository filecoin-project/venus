package vf3

import (
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/filecoin-project/go-f3/gpbft"
	"github.com/filecoin-project/go-f3/manifest"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/venus-shared/actors/policy"
)

type Config struct {
	// BaseNetworkName is the base from which dynamic network names are defined and is usually
	// the name of the network defined by the static manifest. This must be set correctly or,
	// e.g., pubsub topic filters won't work correctly.
	BaseNetworkName gpbft.NetworkName
	// StaticManifest this instance's default manifest absent any dynamic manifests. Also see
	// PrioritizeStaticManifest.
	StaticManifest *manifest.Manifest
	// DynamicManifestProvider is the peer ID of the peer authorized to send us dynamic manifest
	// updates. Dynamic manifest updates can be used for testing but will not be used to affect
	// finality.
	DynamicManifestProvider peer.ID
	// PrioritizeStaticManifest means that, once we get within one finality of the static
	// manifest's bootstrap epoch we'll switch to it and ignore any further dynamic manifest
	// updates. This exists to enable bootstrapping F3.
	PrioritizeStaticManifest bool
	// TESTINGAllowDynamicFinalize allow dynamic manifests to finalize tipsets. DO NOT ENABLE
	// THIS IN PRODUCTION!
	AllowDynamicFinalize bool
}

// NewManifest constructs a sane F3 manifest based on the passed parameters. This function does not
// look at and/or depend on the nodes build params, etc.
func NewManifest(
	nn gpbft.NetworkName,
	finality, bootstrapEpoch abi.ChainEpoch,
	ecPeriod time.Duration,
	initialPowerTable cid.Cid,
) *manifest.Manifest {
	return &manifest.Manifest{
		ProtocolVersion:   manifest.VersionCapability,
		BootstrapEpoch:    int64(bootstrapEpoch),
		NetworkName:       nn,
		InitialPowerTable: initialPowerTable,
		CommitteeLookback: manifest.DefaultCommitteeLookback,
		CatchUpAlignment:  ecPeriod / 2,
		Gpbft:             manifest.DefaultGpbftConfig,
		EC: manifest.EcConfig{
			Period:                   ecPeriod,
			Finality:                 int64(finality),
			DelayMultiplier:          manifest.DefaultEcConfig.DelayMultiplier,
			BaseDecisionBackoffTable: manifest.DefaultEcConfig.BaseDecisionBackoffTable,
			HeadLookback:             0,
			Finalize:                 true,
		},
		CertificateExchange: manifest.CxConfig{
			ClientRequestTimeout: manifest.DefaultCxConfig.ClientRequestTimeout,
			ServerRequestTimeout: manifest.DefaultCxConfig.ServerRequestTimeout,
			MinimumPollInterval:  ecPeriod,
			MaximumPollInterval:  4 * ecPeriod,
		},
	}
}

// NewConfig creates a new F3 config based on the node's build parameters and the passed network
// name.
func NewConfig(nn string, netCfg *config.NetworkParamsConfig) (*Config, error) {
	// Use "filecoin" as the network name on mainnet, otherwise use the network name. Yes,
	// mainnet is called testnetnet in state.
	if nn == "testnetnet" {
		nn = "filecoin"
	}
	manifestServerID, err := peer.Decode(netCfg.ManifestServerID)
	if err != nil {
		return nil, err
	}
	c := &Config{
		BaseNetworkName:          gpbft.NetworkName(nn),
		PrioritizeStaticManifest: true,
		DynamicManifestProvider:  manifestServerID,
		AllowDynamicFinalize:     false,
	}
	if netCfg.F3BootstrapEpoch >= 0 {
		// todo:
		c.StaticManifest = NewManifest(
			c.BaseNetworkName,
			policy.ChainFinality,
			netCfg.F3BootstrapEpoch,
			time.Duration(netCfg.BlockDelay)*time.Second,
			netCfg.F3InitialPowerTableCID,
		)
	}
	return c, nil
}
