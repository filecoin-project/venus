package networks

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/config"
)

func Calibration() *NetworkConf {
	return &NetworkConf{
		Bootstrap: config.BootstrapConfig{
			Addresses: []string{
				"/dns4/bootstrap-0.calibration.fildev.network/tcp/1347/p2p/12D3KooWK1QYsm6iqyhgH7vqsbeoNoKHbT368h1JLHS1qYN36oyc",
				"/dns4/bootstrap-1.calibration.fildev.network/tcp/1347/p2p/12D3KooWKDyJZoPsNak1iYNN1GGmvGnvhyVbWBL6iusYfP3RpgYs",
				"/dns4/bootstrap-2.calibration.fildev.network/tcp/1347/p2p/12D3KooWJRSTnzABB6MYYEBbSTT52phQntVD1PpRTMh1xt9mh6yH",
				"/dns4/bootstrap-3.calibration.fildev.network/tcp/1347/p2p/12D3KooWQLi3kY6HnMYLUtwCe26zWMdNhniFgHVNn1DioQc7NiWv",
			},
			MinPeerThreshold: 0,
			Period:           "30s",
		},
		Network: config.NetworkParamsConfig{
			BlockDelay:             30,
			ConsensusMinerMinPower: 10 << 30,
			ReplaceProofTypes: []int64{
				int64(abi.RegisteredSealProof_StackedDrg512MiBV1),
				int64(abi.RegisteredSealProof_StackedDrg32GiBV1),
				int64(abi.RegisteredSealProof_StackedDrg64GiBV1),
			},
			ForkUpgradeParam: &config.ForkUpgradeConfig{
				UpgradeBreezeHeight:   -1,
				UpgradeSmokeHeight:    -2,
				UpgradeIgnitionHeight: -3,
				UpgradeRefuelHeight:   -4,
				UpgradeActorsV2Height: 30,
				UpgradeTapeHeight:     60,
				UpgradeLiftoffHeight:  -5,
				UpgradeKumquatHeight:  90,
				UpgradeCalicoHeight:   92000,
				UpgradePersianHeight:  92000 + (120 * 60),
				UpgradeOrangeHeight:   250666, // 2021-01-17T19:00:00Z
				UpgradeActorsV3Height: 278026, // 2021-01-27T07:00:00Z

				BreezeGasTampingDuration: 120,
				UpgradeClausHeight:       161386, // 2020-12-17T19:00:00Z

			},
			DrandSchedule:  map[abi.ChainEpoch]config.DrandEnum{0: 1},
			AddressNetwork: address.Testnet,
		},
	}
}
