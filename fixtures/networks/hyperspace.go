package networks

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/venus-shared/types"
)

func HyperspaceNet() *NetworkConf {
	return &NetworkConf{
		Bootstrap: config.BootstrapConfig{
			Addresses: []string{
				"/dns4/de0.bootstrap.hyperspace.yoga/tcp/31000/p2p/12D3KooWRiwg6EHAJMR5w3DZTgpS5W4ncWPSVP2Mr1o4ey1RYSQo",
				"/dns4/de1.bootstrap.hyperspace.yoga/tcp/31000/p2p/12D3KooWM9HZsp1bh5jNu2m9FBSbkKSeSWUPPuDBQiiMfPDBAK3U",
				"/dns4/au0.bootstrap.hyperspace.yoga/tcp/31000/p2p/12D3KooWLup1gTdG9ipt3bSUyPCmM4CT86p9nNe12oqrCX8Zo8Na",
				"/dns4/ca0.bootstrap.hyperspace.yoga/tcp/31000/p2p/12D3KooWNJ4evKioh6gexD4fyvyeFecNtp2oTEPTyp3jtSQ3pWaP",
				"/dns4/sg0.bootstrap.hyperspace.yoga/tcp/31000/p2p/12D3KooWCENec46HHByaJKzbjSqz9TqVdSxSAdi9FKNwdMvfw3vp",
			},
			Period: "30s",
		},
		Network: config.NetworkParamsConfig{
			DevNet:                true,
			NetworkType:           types.NetworkHyperspace,
			GenesisNetworkVersion: network.Version18,
			ReplaceProofTypes: []abi.RegisteredSealProof{
				abi.RegisteredSealProof_StackedDrg512MiBV1,
				abi.RegisteredSealProof_StackedDrg32GiBV1,
				abi.RegisteredSealProof_StackedDrg64GiBV1,
			},
			BlockDelay:              30,
			ConsensusMinerMinPower:  16 << 30,
			MinVerifiedDealSize:     1 << 20,
			PreCommitChallengeDelay: abi.ChainEpoch(10),
			ForkUpgradeParam: &config.ForkUpgradeConfig{
				BreezeGasTampingDuration:    120,
				UpgradeBreezeHeight:         -1,
				UpgradeSmokeHeight:          -2,
				UpgradeIgnitionHeight:       -3,
				UpgradeRefuelHeight:         -4,
				UpgradeAssemblyHeight:       -5,
				UpgradeTapeHeight:           -6,
				UpgradeLiftoffHeight:        -7,
				UpgradeKumquatHeight:        -8,
				UpgradeCalicoHeight:         -9,
				UpgradePersianHeight:        -10,
				UpgradeOrangeHeight:         -11,
				UpgradeClausHeight:          -12,
				UpgradeTrustHeight:          -13,
				UpgradeNorwegianHeight:      -14,
				UpgradeTurboHeight:          -15,
				UpgradeHyperdriveHeight:     -16,
				UpgradeChocolateHeight:      -17,
				UpgradeOhSnapHeight:         -18,
				UpgradeSkyrHeight:           -19,
				UpgradeSharkHeight:          -20,
				UpgradeHyggeHeight:          -21,
				UpgradeLightningHeight:      -22,
				UpgradeThunderHeight:        -23,
				UpgradeWatermelonHeight:     200,
				UpgradeWatermelonFixHeight:  -100, // This fix upgrade only ran on calibrationnet
				UpgradeWatermelonFix2Height: -101, // This fix upgrade only ran on calibrationnet
			},
			DrandSchedule:           map[abi.ChainEpoch]config.DrandEnum{0: 1},
			AddressNetwork:          address.Testnet,
			PropagationDelaySecs:    6,
			AllowableClockDriftSecs: 1,
			Eip155ChainID:           3141,
			ActorDebugging:          false,
		},
	}
}
