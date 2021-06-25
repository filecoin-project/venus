package networks

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/constants"

	"github.com/filecoin-project/venus/pkg/config"
)

func InteropNet() *NetworkConf {
	return &NetworkConf{
		Bootstrap: config.BootstrapConfig{
			Addresses: []string{
				"/dns4/bootstrap-0.interop.fildev.network/tcp/1347/p2p/12D3KooWN86wA54r3v9M8bBYbc1vK9W1ehHDxVGPRaoeUYuXF8R7",
				"/dns4/bootstrap-1.interop.fildev.network/tcp/1347/p2p/12D3KooWNZ41kev8mtBZgWe43qam1VX9pJyf87jnaisQP2urZZ2M",
			},
			MinPeerThreshold: 0,
			Period:           "30s",
		},
		Network: config.NetworkParamsConfig{
			DevNet: true,
			ReplaceProofTypes: []abi.RegisteredSealProof{
				abi.RegisteredSealProof_StackedDrg2KiBV1,
				abi.RegisteredSealProof_StackedDrg8MiBV1,
				abi.RegisteredSealProof_StackedDrg512MiBV1,
			},
			NetworkType:            constants.NetworkInterop,
			BlockDelay:             30,
			ConsensusMinerMinPower: 2048,
			ForkUpgradeParam: &config.ForkUpgradeConfig{
				UpgradeBreezeHeight:     -1,
				UpgradeSmokeHeight:      -1,
				UpgradeIgnitionHeight:   -2,
				UpgradeRefuelHeight:     -3,
				UpgradeAssemblyHeight:   -5,
				UpgradeTapeHeight:       -4,
				UpgradeLiftoffHeight:    -6,
				UpgradeKumquatHeight:    -7,
				UpgradeCalicoHeight:     -8,
				UpgradePersianHeight:    -9,
				UpgradeOrangeHeight:     -10,
				UpgradeTrustHeight:      -12,
				UpgradeNorwegianHeight:  -13,
				UpgradeTurboHeight:      -14,
				UpgradeHyperdriveHeight: -15,

				BreezeGasTampingDuration: 0,
				UpgradeClausHeight:       -11,
			},
			DrandSchedule:           map[abi.ChainEpoch]config.DrandEnum{0: 1},
			AddressNetwork:          address.Testnet,
			PreCommitChallengeDelay: abi.ChainEpoch(10),
		},
	}
}
