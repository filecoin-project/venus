package networks

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/venus-shared/types"
)

func ButterflySnapNet() *NetworkConf {
	return &NetworkConf{
		Bootstrap: config.BootstrapConfig{
			Addresses: []string{
				"/dns4/bootstrap-0.butterfly.fildev.network/tcp/1347/p2p/12D3KooWSUZhAY3eyoPUboJ1ZWe4dNPFWTr1EPoDjbTDSAN15uhY",
				"/dns4/bootstrap-1.butterfly.fildev.network/tcp/1347/p2p/12D3KooWDfvNrSRVGWAGbn3sm9C8z98W2x25qCZjaXGHXmGiH24e",
			},
			MinPeerThreshold: 0,
			Period:           "30s",
		},
		Network: config.NetworkParamsConfig{
			DevNet:                true,
			NetworkType:           types.NetworkButterfly,
			GenesisNetworkVersion: network.Version15,
			ReplaceProofTypes: []abi.RegisteredSealProof{
				abi.RegisteredSealProof_StackedDrg512MiBV1,
				abi.RegisteredSealProof_StackedDrg32GiBV1,
				abi.RegisteredSealProof_StackedDrg64GiBV1,
			},
			BlockDelay:              30,
			ConsensusMinerMinPower:  2 << 30,
			MinVerifiedDealSize:     1 << 20,
			PreCommitChallengeDelay: abi.ChainEpoch(150),
			ForkUpgradeParam: &config.ForkUpgradeConfig{
				UpgradeBreezeHeight:     -1,
				UpgradeSmokeHeight:      -2,
				UpgradeIgnitionHeight:   -3,
				UpgradeRefuelHeight:     -4,
				UpgradeAssemblyHeight:   -5,
				UpgradeTapeHeight:       -6,
				UpgradeLiftoffHeight:    -7,
				UpgradeKumquatHeight:    -8,
				UpgradeCalicoHeight:     -9,
				UpgradePersianHeight:    -10,
				UpgradeOrangeHeight:     -12,
				UpgradeTrustHeight:      -13,
				UpgradeNorwegianHeight:  -14,
				UpgradeTurboHeight:      -15,
				UpgradeHyperdriveHeight: -16,
				UpgradeChocolateHeight:  -17,
				UpgradeOhSnapHeight:     -18,
				UpgradeSkyrHeight:       50,

				BreezeGasTampingDuration: 120,
				UpgradeClausHeight:       -11,
			},
			DrandSchedule:  map[abi.ChainEpoch]config.DrandEnum{0: 1},
			AddressNetwork: address.Testnet,
		},
	}
}
