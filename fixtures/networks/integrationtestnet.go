package networks

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/venus-shared/types"
)

func IntegrationNet() *NetworkConf {

	return &NetworkConf{
		Bootstrap: config.BootstrapConfig{
			Addresses:        []string{},
			MinPeerThreshold: 0,
			Period:           "30s",
		},
		Network: config.NetworkParamsConfig{
			NetworkType:           types.Integrationnet,
			GenesisNetworkVersion: network.Version0,
			ReplaceProofTypes: []abi.RegisteredSealProof{
				abi.RegisteredSealProof_StackedDrg32GiBV1,
				abi.RegisteredSealProof_StackedDrg64GiBV1,
			},
			BlockDelay:              30,
			ConsensusMinerMinPower:  10 << 40,
			MinVerifiedDealSize:     1 << 20,
			PreCommitChallengeDelay: abi.ChainEpoch(150),
			ForkUpgradeParam: &config.ForkUpgradeConfig{
				UpgradeBreezeHeight:     41280,
				UpgradeSmokeHeight:      51000,
				UpgradeIgnitionHeight:   94000,
				UpgradeRefuelHeight:     130800,
				UpgradeAssemblyHeight:   138720,
				UpgradeTapeHeight:       140760,
				UpgradeLiftoffHeight:    148888,
				UpgradeKumquatHeight:    170000,
				UpgradeCalicoHeight:     265200,
				UpgradePersianHeight:    265200 + (120 * 60),
				UpgradeOrangeHeight:     336458,
				UpgradeTrustHeight:      550321,
				UpgradeNorwegianHeight:  665280,
				UpgradeTurboHeight:      712320,
				UpgradeHyperdriveHeight: 892800,
				UpgradeChocolateHeight:  1231620,
				UpgradeOhSnapHeight:     1594680,
				UpgradeSkyrHeight:       99999999999999,

				BreezeGasTampingDuration: 120,
				UpgradeClausHeight:       343200,
			},
			DrandSchedule:  map[abi.ChainEpoch]config.DrandEnum{0: 5, 51000: 1},
			AddressNetwork: address.Testnet,
		},
	}
}
