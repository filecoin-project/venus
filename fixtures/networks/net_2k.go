package networks

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/venus-shared/types"
)

func Net2k() *NetworkConf {
	return &NetworkConf{
		Bootstrap: config.BootstrapConfig{
			Addresses:        []string{},
			MinPeerThreshold: 0,
			Period:           "30s",
		},
		Network: config.NetworkParamsConfig{
			DevNet:                true,
			NetworkType:           types.Network2k,
			GenesisNetworkVersion: network.Version18,
			ReplaceProofTypes: []abi.RegisteredSealProof{
				abi.RegisteredSealProof_StackedDrg2KiBV1,
				abi.RegisteredSealProof_StackedDrg8MiBV1,
			},
			BlockDelay:              4,
			ConsensusMinerMinPower:  2048,
			MinVerifiedDealSize:     256,
			PreCommitChallengeDelay: abi.ChainEpoch(10),
			ForkUpgradeParam: &config.ForkUpgradeConfig{
				BreezeGasTampingDuration: 0,
				UpgradeBreezeHeight:      -1,
				UpgradeSmokeHeight:       -2,
				UpgradeIgnitionHeight:    -3,
				UpgradeRefuelHeight:      -4,
				UpgradeTapeHeight:        -5,
				UpgradeAssemblyHeight:    -6,
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
			},
			DrandSchedule:        map[abi.ChainEpoch]config.DrandEnum{0: 1},
			AddressNetwork:       address.Testnet,
			PropagationDelaySecs: 1,
			Eip155ChainID:        31415926,
		},
	}
}
