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
				"/dns4/de0.bootstrap.wallaby.network/tcp/1337/p2p/12D3KooWHAvUVk5XuxSwi2dNLWbTDDRSGeHxMuWdQ3SQpRuNHbLz",
				"/dns4/de1.bootstrap.wallaby.network/tcp/1337/p2p/12D3KooWBRqtxhJCtiLmCwKgAQozJtdGinEDdJGoS5oHw7vCjMGc",
				"/dns4/ca0.bootstrap.wallaby.network/tcp/1337/p2p/12D3KooWCApBpUk7EX9pmEfyky1gKC6N2KJ74S1AwFfvnkDqw3pK",
				"/dns4/sg0.bootstrap.wallaby.network/tcp/1337/p2p/12D3KooWLnYqr4hRoNHBJQVXsFGkDoKuoVfw5R2ASw1bHzrWU5Px",
			},
			MinPeerThreshold: 0,
			Period:           "30s",
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
			},
			DrandSchedule:        map[abi.ChainEpoch]config.DrandEnum{0: 1},
			AddressNetwork:       address.Testnet,
			PropagationDelaySecs: 6,
			Eip155ChainID:        31415,
			ActorDebugging:       true,
		},
	}
}
