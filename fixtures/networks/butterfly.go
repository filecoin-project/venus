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

func ButterflySnapNet() *NetworkConf {
	nc := &NetworkConf{
		Bootstrap: config.BootstrapConfig{
			Addresses: []string{
				"/dnsaddr/bootstrap.butterfly.fildev.network",
			},
			Period: "30s",
		},
		Network: config.NetworkParamsConfig{
			DevNet:                true,
			NetworkType:           types.NetworkButterfly,
			GenesisNetworkVersion: network.Version24,
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
				BreezeGasTampingDuration:             120,
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
				UpgradeTeepHeight:                    100,
				UpgradeTockFixHeight:                 -29,
			},
			DrandSchedule:           map[abi.ChainEpoch]config.DrandEnum{0: config.DrandQuicknet},
			AddressNetwork:          address.Testnet,
			PropagationDelaySecs:    6,
			AllowableClockDriftSecs: 1,
			Eip155ChainID:           3141592,
			ActorDebugging:          false,
			F3Enabled:               true,

			UpgradeTeepInitialFilReserved: constants.WholeFIL(1_600_000_000), // FIP-0100: 300M -> 1.6B FIL
		},
	}

	nc.Network.ForkUpgradeParam.UpgradeTockHeight = nc.Network.ForkUpgradeParam.UpgradeTeepHeight + builtin.EpochsInDay*2

	return nc
}
