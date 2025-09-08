package networks

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/venus-shared/types"
)

func Net2k() *NetworkConf {
	nc := &NetworkConf{
		Bootstrap: config.BootstrapConfig{
			Addresses: []string{},
			Period:    "30s",
		},
		Network: config.NetworkParamsConfig{
			DevNet:                true,
			NetworkType:           types.Network2k,
			GenesisNetworkVersion: network.Version26,
			ReplaceProofTypes: []abi.RegisteredSealProof{
				abi.RegisteredSealProof_StackedDrg2KiBV1,
				abi.RegisteredSealProof_StackedDrg8MiBV1,
			},
			BlockDelay:              4,
			ConsensusMinerMinPower:  2048,
			MinVerifiedDealSize:     256,
			PreCommitChallengeDelay: abi.ChainEpoch(10),
			ForkUpgradeParam: &config.ForkUpgradeConfig{
				BreezeGasTampingDuration:             0,
				UpgradeBreezeHeight:                  -1,
				UpgradeSmokeHeight:                   -2,
				UpgradeIgnitionHeight:                -3,
				UpgradeRefuelHeight:                  -4,
				UpgradeTapeHeight:                    -5,
				UpgradeAssemblyHeight:                -6,
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
				UpgradeTuktukPowerRampDurationEpochs: 200,
				UpgradeTuktukHeight:                  -28,
				UpgradeTeepHeight:                    -30,
				UpgradeTockHeight:                    -31,
				UpgradeTockFixHeight:                 -29,
				UpgradeGoldenWeekHeight:              200,
			},
			DrandSchedule:           map[abi.ChainEpoch]config.DrandEnum{0: config.DrandQuicknet},
			AddressNetwork:          address.Testnet,
			PropagationDelaySecs:    1,
			AllowableClockDriftSecs: 1,
			Eip155ChainID:           31415926,
			ActorDebugging:          true,
			F3Enabled:               true,

			UpgradeTeepInitialFilReserved: constants.WholeFIL(1_400_000_000), // FIP-0100: 300M -> 1.4B FIL
		},
	}

	return nc
}
