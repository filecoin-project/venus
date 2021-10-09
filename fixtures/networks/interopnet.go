package networks

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/specs-actors/v6/actors/builtin/miner"
	"github.com/filecoin-project/venus/pkg/constants"
	"math"

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
			GenesisNetworkVersion:  network.Version0,
			BlockDelay:             30,
			ConsensusMinerMinPower: 2048,
			MinVerifiedDealSize:    256,
			ForkUpgradeParam: &config.ForkUpgradeConfig{
				UpgradeBreezeHeight:        -1,
				UpgradeSmokeHeight:         -1,
				UpgradeIgnitionHeight:      -2,
				UpgradeRefuelHeight:        -3,
				UpgradeAssemblyHeight:      -5,
				UpgradeTapeHeight:          -4,
				UpgradeLiftoffHeight:       -6,
				UpgradeKumquatHeight:       -7,
				UpgradePriceListOopsHeight: -8,
				UpgradeCalicoHeight:        -9,
				UpgradePersianHeight:       -10,
				UpgradeOrangeHeight:        -11,
				UpgradeTrustHeight:         -13,
				UpgradeNorwegianHeight:     -14,
				UpgradeTurboHeight:         -15,
				UpgradeHyperdriveHeight:    -16,
				UpgradeChocolateHeight:     math.MaxInt32,

				BreezeGasTampingDuration: 0,
				UpgradeClausHeight:       -11,
			},
			DrandSchedule:           map[abi.ChainEpoch]config.DrandEnum{0: 1},
			AddressNetwork:          address.Testnet,
			PreCommitChallengeDelay: abi.ChainEpoch(10),
			FaultMaxAge:             miner.WPoStProvingPeriod * 42,
		},
	}
}
