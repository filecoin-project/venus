package networks

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/venus-shared/types"
)

func WallabyNet() *NetworkConf {
	return &NetworkConf{
		Bootstrap: config.BootstrapConfig{
			Addresses: []string{
				"/dns4/de0.bootstrap.wallaby.yoga/tcp/41000/p2p/12D3KooWGXLjN4FCXyTsvLPUrbZfkA5p7gXJ11WXXB56cQLEmNkE",
				"/dns4/de1.bootstrap.wallaby.yoga/tcp/41000/p2p/12D3KooWEHyGpfQsLMPhCo4zmfp6uZfhQisiWZMYaPu1j92d2dax",
				"/dns4/au0.bootstrap.wallaby.yoga/tcp/41000/p2p/12D3KooWLEPk3TcgpD7aWoou4dzbgdQA14Y9eTCg9rcoLaLruHtf",
				"/dns4/ca0.bootstrap.wallaby.yoga/tcp/41000/p2p/12D3KooWQAupDxeHoLzmc617FzhWnHHWEt8e2fNfccqByT5mHWPp",
				"/dns4/sg0.bootstrap.wallaby.yoga/tcp/41000/p2p/12D3KooWAkSaZCXSngvgSi4ufVModcExCysnS3JhG6nnprPjVV4o",
			},
			Period: "30s",
		},
		Network: config.NetworkParamsConfig{
			DevNet:                true,
			NetworkType:           types.NetworkWallaby,
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
			Eip155ChainID:           31415,
			ActorDebugging:          false,
		},
	}
}
