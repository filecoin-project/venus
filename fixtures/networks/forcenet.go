package networks

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
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
			BlockDelay:             30,
			ConsensusMinerMinPower: 2048,
			ForkUpgradeParam: &config.ForkUpgradeConfig{
				UpgradeBreezeHeight:   -1,
				UpgradeSmokeHeight:    -1,
				UpgradeIgnitionHeight: -2,
				UpgradeRefuelHeight:   -3,
				UpgradeTapeHeight:     -4,
				UpgradeLiftoffHeight:  -5,
				// This signals our tentative epoch for mainnet launch. Can make it later, but not earlier.
				// Miners, clients, developers, custodians all need time to prepare.
				// We still have upgrades and state changes to do, but can happen after signaling timing here.

				UpgradeAssemblyHeight:      10, // critical: the network can bootstrap from v1 only
				UpgradeKumquatHeight:       15,
				UpgradePriceListOopsHeight: 99,
				UpgradeCalicoHeight:        20,
				UpgradePersianHeight:       25,
				UpgradeOrangeHeight:        27,
				UpgradeClausHeight:         30,
				UpgradeTrustHeight:         35,
				UpgradeNorwegianHeight:     40,
				UpgradeTurboHeight:         45,
				UpgradeHyperdriveHeight:    50,

				BreezeGasTampingDuration: 0,
			},
			DrandSchedule:           map[abi.ChainEpoch]config.DrandEnum{0: config.DrandMainnet},
			AddressNetwork:          address.Testnet,
			PreCommitChallengeDelay: abi.ChainEpoch(10),
		},
	}
}
