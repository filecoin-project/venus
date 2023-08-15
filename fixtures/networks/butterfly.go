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
				"/dns4/bootstrap-0.butterfly.fildev.network/tcp/1347/p2p/12D3KooWD5mtdmjHQ1Puj9Md7SEfoa7kWMpwqUhAKsyYsBP56LQC",
				"/dns4/bootstrap-1.butterfly.fildev.network/tcp/1347/p2p/12D3KooWEoYPkm6o87ES6AppFY7d7WHJUQg7XVPRAyQZjEU31efQ",
			},
			Period: "30s",
		},
		Network: config.NetworkParamsConfig{
			DevNet:                true,
			NetworkType:           types.NetworkButterfly,
			GenesisNetworkVersion: network.Version18,
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
				BreezeGasTampingDuration: 120,
				UpgradeBreezeHeight:      -1,
				UpgradeSmokeHeight:       -2,
				UpgradeIgnitionHeight:    -3,
				UpgradeRefuelHeight:      -4,
				UpgradeAssemblyHeight:    -5,
				UpgradeTapeHeight:        -6,
				UpgradeLiftoffHeight:     -7,
				UpgradeKumquatHeight:     -8,
				UpgradeCalicoHeight:      -9,
				UpgradePersianHeight:     -10,
				UpgradeOrangeHeight:      -11,
				UpgradeClausHeight:       -12,
				UpgradeTrustHeight:       -13,
				UpgradeNorwegianHeight:   -14,
				UpgradeTurboHeight:       -15,
				UpgradeHyperdriveHeight:  -16,
				UpgradeChocolateHeight:   -17,
				UpgradeOhSnapHeight:      -18,
				UpgradeSkyrHeight:        -19,
				UpgradeSharkHeight:       -20,
				UpgradeHyggeHeight:       -21,
				// TODO: set upgrade height
				UpgradeLightningHeight: 50,
				UpgradeThunderHeight:   50 + 360,
			},
			DrandSchedule:           map[abi.ChainEpoch]config.DrandEnum{0: 1},
			AddressNetwork:          address.Testnet,
			PropagationDelaySecs:    6,
			AllowableClockDriftSecs: 1,
			Eip155ChainID:           3141592,
			ActorDebugging:          false,
		},
	}
}
