package networks

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
)

func ForceNet() *NetworkConf {
	return &NetworkConf{
		Bootstrap: config.BootstrapConfig{
			Addresses: []string{},

			MinPeerThreshold: 0,
			Period:           "30s",
		},
		Network: config.NetworkParamsConfig{
			DevNet: true,
			ReplaceProofTypes: []abi.RegisteredSealProof{
				abi.RegisteredSealProof_StackedDrg8MiBV1,
				abi.RegisteredSealProof_StackedDrg512MiBV1,
				abi.RegisteredSealProof_StackedDrg32GiBV1,
			},
			NetworkType:            constants.NetworkForce,
			GenesisNetworkVersion:  network.Version14,
			BlockDelay:             30,
			ConsensusMinerMinPower: 2048,
			ForkUpgradeParam: &config.ForkUpgradeConfig{
				UpgradeBreezeHeight:      -1,
				BreezeGasTampingDuration: 0,
				UpgradeSmokeHeight:       -1,
				UpgradeIgnitionHeight:    -2,
				UpgradeRefuelHeight:      -3,
				UpgradeTapeHeight:        -4,
				UpgradeLiftoffHeight:     -6,
				// This signals our tentative epoch for mainnet launch. Can make it later, but not earlier.
				// Miners, clients, developers, custodians all need time to prepare.
				// We still have upgrades and state changes to do, but can happen after signaling timing here.

				UpgradeAssemblyHeight:      -5, // critical: the network can bootstrap from v1 only
				UpgradeKumquatHeight:       -7,
				UpgradePriceListOopsHeight: -8,
				UpgradeCalicoHeight:        -9,
				UpgradePersianHeight:       -10,
				UpgradeOrangeHeight:        -11,
				UpgradeClausHeight:         -12,
				UpgradeTrustHeight:         -13,
				UpgradeNorwegianHeight:     -14,
				UpgradeTurboHeight:         -15,
				UpgradeHyperdriveHeight:    -16,
				UpgradeChocolateHeight:     -17,
				UpgradeSnapDealsHeight:     -18,
			},
			DrandSchedule:           map[abi.ChainEpoch]config.DrandEnum{0: config.DrandMainnet},
			AddressNetwork:          address.Testnet,
			PreCommitChallengeDelay: abi.ChainEpoch(10),
		},
	}
}
