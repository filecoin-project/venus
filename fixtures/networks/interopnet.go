package networks

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/venus-shared/types"
)

func InteropNet() *NetworkConf {
	return &NetworkConf{
		Bootstrap: config.BootstrapConfig{
			Addresses: []string{
				"/dns4/bootstrap-0.interop.fildev.network/tcp/1347/p2p/12D3KooWDY249nj6gxSiBTocNBnkbBBvnfPgkxq5SBUVrccjwRnr",
				"/dns4/bootstrap-1.interop.fildev.network/tcp/1347/p2p/12D3KooWCwFsKt4NyoJQk7vjiACXL9LznBWANXKjCebKJ7MHh3h4",
			},
			MinPeerThreshold: 0,
			Period:           "30s",
		},
		Network: config.NetworkParamsConfig{
			DevNet:                true,
			NetworkType:           types.NetworkInterop,
			GenesisNetworkVersion: network.Version15,
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
				UpgradeBreezeHeight:     -1,
				UpgradeSmokeHeight:      -2,
				UpgradeIgnitionHeight:   -3,
				UpgradeRefuelHeight:     -4,
				UpgradeAssemblyHeight:   -5,
				UpgradeTapeHeight:       -6,
				UpgradeLiftoffHeight:    -7,
				UpgradeKumquatHeight:    -8,
				UpgradeCalicoHeight:     -9,
				UpgradePersianHeight:    -10,
				UpgradeOrangeHeight:     -11,
				UpgradeTrustHeight:      -13,
				UpgradeNorwegianHeight:  -14,
				UpgradeTurboHeight:      -15,
				UpgradeHyperdriveHeight: -16,
				UpgradeChocolateHeight:  -17,
				UpgradeOhSnapHeight:     -18,
				UpgradeSkyrHeight:       100,

				BreezeGasTampingDuration: 0,
				UpgradeClausHeight:       -11,
			},
			DrandSchedule:  map[abi.ChainEpoch]config.DrandEnum{0: 1},
			AddressNetwork: address.Testnet,
		},
	}
}
