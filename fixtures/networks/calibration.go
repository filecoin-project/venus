package networks

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
)

type NetworkConf struct {
	Bootstrap config.BootstrapConfig
	Network   config.NetworkParamsConfig
}

func Calibration() *NetworkConf {
	return &NetworkConf{
		Bootstrap: config.BootstrapConfig{
			Addresses: []string{
				"/dns4/bootstrap-0.calibration.fildev.network/tcp/1347/p2p/12D3KooWJkikQQkxS58spo76BYzFt4fotaT5NpV2zngvrqm4u5ow",
				"/dns4/bootstrap-1.calibration.fildev.network/tcp/1347/p2p/12D3KooWLce5FDHR4EX4CrYavphA5xS3uDsX6aoowXh5tzDUxJav",
				"/dns4/bootstrap-2.calibration.fildev.network/tcp/1347/p2p/12D3KooWA9hFfQG9GjP6bHeuQQbMD3FDtZLdW1NayxKXUT26PQZu",
				"/dns4/bootstrap-3.calibration.fildev.network/tcp/1347/p2p/12D3KooWMHDi3LVTFG8Szqogt7RkNXvonbQYqSazxBx41A5aeuVz",
			},
			MinPeerThreshold: 0,
			Period:           "30s",
		},
		Network: config.NetworkParamsConfig{
			DevNet:                 true,
			NetworkType:            constants.NetworkCalibnet,
			GenesisNetworkVersion:  network.Version0,
			BlockDelay:             30,
			ConsensusMinerMinPower: 32 << 30,
			ReplaceProofTypes: []abi.RegisteredSealProof{
				abi.RegisteredSealProof_StackedDrg32GiBV1,
				abi.RegisteredSealProof_StackedDrg64GiBV1,
			},
			ForkUpgradeParam: &config.ForkUpgradeConfig{
				UpgradeBreezeHeight:     -1,
				UpgradeSmokeHeight:      -2,
				UpgradeIgnitionHeight:   -3,
				UpgradeRefuelHeight:     -4,
				UpgradeAssemblyHeight:   30,
				UpgradeTapeHeight:       60,
				UpgradeLiftoffHeight:    -5,
				UpgradeKumquatHeight:    90,
				UpgradeCalicoHeight:     120,
				UpgradePersianHeight:    100 + (120 * 1),
				UpgradeOrangeHeight:     300,
				UpgradeTrustHeight:      330,
				UpgradeNorwegianHeight:  360,
				UpgradeTurboHeight:      390,
				UpgradeHyperdriveHeight: 420,

				BreezeGasTampingDuration: 120,
				UpgradeClausHeight:       270,
				UpgradeChocolateHeight:   312746,
				UpgradeOhSnapHeight:      682006, // 2022-02-10T19:23:00Z
			},
			DrandSchedule:           map[abi.ChainEpoch]config.DrandEnum{0: 1},
			AddressNetwork:          address.Testnet,
			PreCommitChallengeDelay: abi.ChainEpoch(150),
		},
	}
}
