package networks

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
	"github.com/filecoin-project/venus/venus-shared/types"
)

func InteropNet() *NetworkConf {
	nc := &NetworkConf{
		Bootstrap: config.BootstrapConfig{
			Addresses: []string{
				"/dns4/bootstrap-0.interop.fildev.network/tcp/1347/p2p/12D3KooWDpppr8csCNvEPnD2Z83KTPdBTM7iJhL66qK8LK3bB5NU",
				"/dns4/bootstrap-1.interop.fildev.network/tcp/1347/p2p/12D3KooWR3K1sXWoDYcXWqDF26mFEM1o1g7e7fcVR3NYE7rn24Gs",
			},
			Period: "30s",
		},
		Network: config.NetworkParamsConfig{
			DevNet:                true,
			NetworkType:           types.NetworkInterop,
			GenesisNetworkVersion: network.Version22,
			ReplaceProofTypes: []abi.RegisteredSealProof{
				abi.RegisteredSealProof_StackedDrg2KiBV1,
				abi.RegisteredSealProof_StackedDrg8MiBV1,
				abi.RegisteredSealProof_StackedDrg512MiBV1,
			},
			BlockDelay:              30,
			ConsensusMinerMinPower:  2048,
			MinVerifiedDealSize:     256,
			PreCommitChallengeDelay: abi.ChainEpoch(10),
			ForkUpgradeParam: &config.ForkUpgradeConfig{
				BreezeGasTampingDuration:             0,
				UpgradeBreezeHeight:                  -1,
				UpgradeSmokeHeight:                   -2,
				UpgradeIgnitionHeight:                -3,
				UpgradeRefuelHeight:                  -4,
				UpgradeAssemblyHeight:                -5,
				UpgradeTapeHeight:                    -6,
				UpgradeLiftoffHeight:                 -7,
				UpgradeKumquatHeight:                 -8,
				UpgradeCalicoHeight:                  -9,
				UpgradePersianHeight:                 -10,
				UpgradeOrangeHeight:                  -11,
				UpgradeClausHeight:                   -12,
				UpgradeTrustHeight:                   -13,
				UpgradeNorwegianHeight:               -14,
				UpgradeTurboHeight:                   -15,
				UpgradeHyperdriveHeight:              -16,
				UpgradeChocolateHeight:               -17,
				UpgradeOhSnapHeight:                  -18,
				UpgradeSkyrHeight:                    -19,
				UpgradeSharkHeight:                   -20,
				UpgradeHyggeHeight:                   -21,
				UpgradeLightningHeight:               -22,
				UpgradeThunderHeight:                 -23,
				UpgradeWatermelonHeight:              -24,
				UpgradeWatermelonFixHeight:           -100, // This fix upgrade only ran on calibrationnet
				UpgradeWatermelonFix2Height:          -101, // This fix upgrade only ran on calibrationnet
				UpgradeDragonHeight:                  -25,
				UpgradeCalibrationDragonFixHeight:    -102, // This fix upgrade only ran on calibrationnet
				UpgradePhoenixHeight:                 -26,
				UpgradeWaffleHeight:                  -27,
				UpgradeTuktukHeight:                  -28,
				UpgradeTuktukPowerRampDurationEpochs: builtin.EpochsInYear,
				UpgradeTeepHeight:                    -30,
				UpgradeTockHeight:                    -31,
				UpgradeTockFixHeight:                 -29,
				UpgradeXxHeight:                      50,
			},
			DrandSchedule:           map[abi.ChainEpoch]config.DrandEnum{0: config.DrandQuicknet},
			AddressNetwork:          address.Testnet,
			PropagationDelaySecs:    6,
			AllowableClockDriftSecs: 1,
			Eip155ChainID:           3141592,
			ActorDebugging:          false,
			F3Enabled:               true,

			UpgradeTeepInitialFilReserved: constants.InitialFilReserved, // FIP-0100: no change for interop
		},
	}

	return nc
}
