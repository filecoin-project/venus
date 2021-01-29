package networks

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/venus/pkg/config"
)

func IntegrationNet() *NetworkConf {

	return &NetworkConf{
		Bootstrap: config.BootstrapConfig{
			Addresses:        []string{},
			MinPeerThreshold: 0,
			Period:           "30s",
		},
		Network: config.NetworkParamsConfig{
			BlockDelay:             30,
			ConsensusMinerMinPower: 10 << 40,
			ForkUpgradeParam: &config.ForkUpgradeConfig{
				UpgradeBreezeHeight:   41280,
				UpgradeSmokeHeight:    51000,
				UpgradeIgnitionHeight: 94000,
				UpgradeRefuelHeight:   130800,
				UpgradeActorsV2Height: 138720,
				UpgradeTapeHeight:     140760,
				UpgradeLiftoffHeight:  148888,
				UpgradeKumquatHeight:  170000,
				UpgradeCalicoHeight:   265200,
				UpgradePersianHeight:  265200 + (120 * 60),
				UpgradeOrangeHeight:   336458,
				UpgradeActorsV3Height: -1, // ToDo

				BreezeGasTampingDuration: 120,
				UpgradeClausHeight:       343200,
			},
			DrandSchedule:  map[abi.ChainEpoch]config.DrandEnum{0: 5, 51000: 1},
			AddressNetwork: address.Testnet,
		},
	}
}
