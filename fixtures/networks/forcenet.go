package networks

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/venus-shared/types"
)

func ForceNet() *NetworkConf {
	nc := &NetworkConf{
		Bootstrap: config.BootstrapConfig{
			Addresses: []string{},

			Period: "30s",
		},
		Network: config.NetworkParamsConfig{
			DevNet:                true,
			NetworkType:           types.NetworkForce,
			GenesisNetworkVersion: network.Version26,
			ReplaceProofTypes: []abi.RegisteredSealProof{
				abi.RegisteredSealProof_StackedDrg8MiBV1,
				abi.RegisteredSealProof_StackedDrg512MiBV1,
				abi.RegisteredSealProof_StackedDrg32GiBV1,
			},
			BlockDelay:              30,
			ConsensusMinerMinPower:  2048,
			MinVerifiedDealSize:     256,
			PreCommitChallengeDelay: abi.ChainEpoch(10),
			ForkUpgradeParam: &config.ForkUpgradeConfig{
				UpgradeBreezeHeight:      -1,
				BreezeGasTampingDuration: 0,
				UpgradeSmokeHeight:       -2,
				UpgradeIgnitionHeight:    -3,
				UpgradeRefuelHeight:      -4,
				UpgradeTapeHeight:        -5,
				UpgradeLiftoffHeight:     -6,
				// This signals our tentative epoch for mainnet launch. Can make it later, but not earlier.
				// Miners, clients, developers, custodians all need time to prepare.
				// We still have upgrades and state changes to do, but can happen after signaling timing here.

				UpgradeAssemblyHeight:                -7, // critical: the network can bootstrap from v1 only
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
				UpgradeTuktukPowerRampDurationEpochs: 200,
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
