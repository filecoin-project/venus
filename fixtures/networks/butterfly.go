package networks

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
)

func ButterflySnapNet() *NetworkConf {
	return &NetworkConf{
		Bootstrap: config.BootstrapConfig{
			Addresses: []string{
				"/dns4/bootstrap-0.butterfly.fildev.network/tcp/1347/p2p/12D3KooWBdRCBLUeKvoy22u5DcXs61adFn31v8WWCZgmBjDCjbsC",
				"/dns4/bootstrap-1.butterfly.fildev.network/tcp/1347/p2p/12D3KooWDUQJBA18njjXnG9RtLxoN3muvdU7PEy55QorUEsdAqdy",
			},
			MinPeerThreshold: 0,
			Period:           "30s",
		},
		Network: config.NetworkParamsConfig{
			DevNet: true,
			ReplaceProofTypes: []abi.RegisteredSealProof{
				abi.RegisteredSealProof_StackedDrg512MiBV1,
				abi.RegisteredSealProof_StackedDrg32GiBV1,
				abi.RegisteredSealProof_StackedDrg64GiBV1,
			},
			NetworkType:            constants.NetSnapDeal,
			GenesisNetworkVersion:  network.Version14,
			BlockDelay:             30,
			ConsensusMinerMinPower: 2048,
			MinVerifiedDealSize:    256,
			ForkUpgradeParam: &config.ForkUpgradeConfig{
				UpgradeBreezeHeight:        -1,
				UpgradeSmokeHeight:         -1,
				UpgradeIgnitionHeight:      -2,
				UpgradeRefuelHeight:        -3,
				UpgradeAssemblyHeight:      -5,
				UpgradeTapeHeight:          -4,
				UpgradeLiftoffHeight:       -6,
				UpgradeKumquatHeight:       -7,
				UpgradePriceListOopsHeight: -8,
				UpgradeCalicoHeight:        -9,
				UpgradePersianHeight:       -10,
				UpgradeOrangeHeight:        -11,
				UpgradeTrustHeight:         -13,
				UpgradeNorwegianHeight:     -14,
				UpgradeTurboHeight:         -15,
				UpgradeHyperdriveHeight:    -16,
				UpgradeChocolateHeight:     -17,
				UpgradeOhSnapHeight:        30262,

				BreezeGasTampingDuration: 0,
				UpgradeClausHeight:       -11,
			},
			DrandSchedule:           map[abi.ChainEpoch]config.DrandEnum{0: 1},
			AddressNetwork:          address.Testnet,
			PreCommitChallengeDelay: abi.ChainEpoch(10),
		},
	}
}
