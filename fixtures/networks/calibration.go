package networks

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/venus-shared/types"
)

type NetworkConf struct {
	Bootstrap config.BootstrapConfig
	Network   config.NetworkParamsConfig
}

func Calibration() *NetworkConf {
	return &NetworkConf{
		Bootstrap: config.BootstrapConfig{
			Addresses: []string{
				"/dns4/bootstrap-0.calibration.fildev.network/tcp/1347/p2p/12D3KooWCi2w8U4DDB9xqrejb5KYHaQv2iA2AJJ6uzG3iQxNLBMy",
				"/dns4/bootstrap-1.calibration.fildev.network/tcp/1347/p2p/12D3KooWDTayrBojBn9jWNNUih4nNQQBGJD7Zo3gQCKgBkUsS6dp",
				"/dns4/bootstrap-2.calibration.fildev.network/tcp/1347/p2p/12D3KooWNRxTHUn8bf7jz1KEUPMc2dMgGfa4f8ZJTsquVSn3vHCG",
				"/dns4/bootstrap-3.calibration.fildev.network/tcp/1347/p2p/12D3KooWFWUqE9jgXvcKHWieYs9nhyp6NF4ftwLGAHm4sCv73jjK",
				"/dns4/calibration.node.glif.io/tcp/1237/p2p/12D3KooWQPYouEAsUQKzvFUA9sQ8tz4rfpqtTzh2eL6USd9bwg7x",
			},
			Period: "30s",
		},
		Network: config.NetworkParamsConfig{
			DevNet:                true,
			NetworkType:           types.NetworkCalibnet,
			GenesisNetworkVersion: network.Version0,
			ReplaceProofTypes: []abi.RegisteredSealProof{
				abi.RegisteredSealProof_StackedDrg32GiBV1,
				abi.RegisteredSealProof_StackedDrg64GiBV1,
			},
			BlockDelay:              30,
			ConsensusMinerMinPower:  32 << 30,
			MinVerifiedDealSize:     1 << 20,
			PreCommitChallengeDelay: abi.ChainEpoch(150),
			ForkUpgradeParam: &config.ForkUpgradeConfig{
				BreezeGasTampingDuration:    120,
				UpgradeBreezeHeight:         -1,
				UpgradeSmokeHeight:          -2,
				UpgradeIgnitionHeight:       -3,
				UpgradeRefuelHeight:         -4,
				UpgradeAssemblyHeight:       30,
				UpgradeTapeHeight:           60,
				UpgradeLiftoffHeight:        -5,
				UpgradeKumquatHeight:        90,
				UpgradeCalicoHeight:         120,
				UpgradePersianHeight:        120 + (builtin2.EpochsInHour * 1),
				UpgradeClausHeight:          270,
				UpgradeOrangeHeight:         300,
				UpgradeTrustHeight:          330,
				UpgradeNorwegianHeight:      360,
				UpgradeTurboHeight:          390,
				UpgradeHyperdriveHeight:     420,
				UpgradeChocolateHeight:      450,
				UpgradeOhSnapHeight:         480,
				UpgradeSkyrHeight:           510,
				UpgradeSharkHeight:          16800,
				UpgradeHyggeHeight:          322354,        // 2023-02-21T16:30:00Z
				UpgradeLightningHeight:      489094,        // 2023-04-20T14:00:00Z
				UpgradeThunderHeight:        489094 + 3120, // 2023-04-21T16:00:00Z
				UpgradeWatermelonHeight:     1013134,       // 2023-10-19T13:00:00Z
				UpgradeWatermelonFixHeight:  1070494,       // 2023-11-07T13:00:00Z
				UpgradeWatermelonFix2Height: 1108174,       // 2023-11-21T13:00:00Z
			},
			DrandSchedule:           map[abi.ChainEpoch]config.DrandEnum{0: 1},
			AddressNetwork:          address.Testnet,
			PropagationDelaySecs:    10,
			AllowableClockDriftSecs: 1,
			Eip155ChainID:           314159,
			ActorDebugging:          false,
		},
	}
}
