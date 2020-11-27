package networks

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/beacon"
	"github.com/filecoin-project/venus/pkg/config"
)

func Net2k() *NetworkConf {
	return &NetworkConf{
		Bootstrap: config.BootstrapConfig{
			Addresses:        []string{},
			MinPeerThreshold: 0,
			Period:           "30s",
		},
		Network: config.NetworkParamsConfig{
			BlockDelay:             4,
			ConsensusMinerMinPower: 2048,
			MinVerifiedDealSize:    256,
			ReplaceProofTypes: []int64{
				int64(abi.RegisteredSealProof_StackedDrg2KiBV1),
			},
			ForkUpgradeParam: config.ForkUpgradeConfig{
				UpgradeSmokeHeight:       -1,
				UpgradeBreezeHeight:      -1,
				UpgradeIgnitionHeight:    -2,
				UpgradeLiftoffHeight:     -5,
				UpgradeActorsV2Height:    10,
				UpgradeRefuelHeight:      -3,
				UpgradeTapeHeight:        -4,
				UpgradeKumquatHeight:     15,
				BreezeGasTampingDuration: 0,
				UpgradeCalicoHeight:      20,
				UpgradePersianHeight:     25,
			},
			DrandSchedule: map[abi.ChainEpoch]beacon.DrandEnum{0: 1},
		},
	}
}
