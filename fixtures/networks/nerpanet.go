package networks

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
)

func NerpaNet() *NetworkConf {
	return &NetworkConf{
		Bootstrap: config.BootstrapConfig{
			Addresses: []string{
				"/dns4/bootstrap-2.nerpa.interplanetary.dev/tcp/1347/p2p/12D3KooWQcL6ReWmR6ASWx4iT7EiAmxKDQpvgq1MKNTQZp5NPnWW",
				"/dns4/bootstrap-0.nerpa.interplanetary.dev/tcp/1347/p2p/12D3KooWGyJCwCm7EfupM15CFPXM4c7zRVHwwwjcuy9umaGeztMX",
				"/dns4/bootstrap-3.nerpa.interplanetary.dev/tcp/1347/p2p/12D3KooWNK9RmfksKXSCQj7ZwAM7L6roqbN4kwJteihq7yPvSgPs",
				"/dns4/bootstrap-1.nerpa.interplanetary.dev/tcp/1347/p2p/12D3KooWCWSaH6iUyXYspYxELjDfzToBsyVGVz3QvC7ysXv7wESo",
			},

			MinPeerThreshold: 0,
			Period:           "30s",
		},
		Network: config.NetworkParamsConfig{
			DevNet: false,
			ReplaceProofTypes: []abi.RegisteredSealProof{
				abi.RegisteredSealProof_StackedDrg512MiBV1,
				abi.RegisteredSealProof_StackedDrg32GiBV1,
				abi.RegisteredSealProof_StackedDrg64GiBV1,
			},
			NetworkType:            constants.NetworkNerpa,
			BlockDelay:             30,
			ConsensusMinerMinPower: 4 << 40,
			ForkUpgradeParam: &config.ForkUpgradeConfig{
				UpgradeBreezeHeight:   -1,
				UpgradeSmokeHeight:    -1,
				UpgradeIgnitionHeight: -2,
				UpgradeRefuelHeight:   -3,
				UpgradeAssemblyHeight: 30,
				UpgradeTapeHeight:     60,
				UpgradeLiftoffHeight:  -5,
				// This signals our tentative epoch for mainnet launch. Can make it later, but not earlier.
				// Miners, clients, developers, custodians all need time to prepare.
				// We still have upgrades and state changes to do, but can happen after signaling timing here.
				UpgradeKumquatHeight:     90,
				UpgradeCalicoHeight:      100,
				UpgradePersianHeight:     100 + (builtin2.EpochsInHour * 1),
				UpgradeOrangeHeight:      300,
				UpgradeTrustHeight:       600,
				BreezeGasTampingDuration: 0,
				UpgradeClausHeight:       250,
				UpgradeNorwegianHeight:   201000,
				UpgradeTurboHeight:       203000,
				UpgradeHyperdriveHeight:  999999999,
			},
			DrandSchedule:           map[abi.ChainEpoch]config.DrandEnum{0: config.DrandMainnet},
			AddressNetwork:          address.Mainnet,
			PreCommitChallengeDelay: abi.ChainEpoch(10),
		},
	}
}
