package networks

import (
	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/venus/internal/pkg/config"
)

func Mainnet() *NetworkConf {
	return &NetworkConf{
		Bootstrap: config.BootstrapConfig{
			Addresses:        []string{},
			MinPeerThreshold: 1,
			Period:           "30s",
		},
		Network: config.NetworkParamsConfig{
			ConsensusMinerMinPower: 2048,
			ReplaceProofTypes: []int64{
				int64(abi.RegisteredSealProof_StackedDrg8MiBV1),
				int64(abi.RegisteredSealProof_StackedDrg512MiBV1),
				int64(abi.RegisteredSealProof_StackedDrg32GiBV1),
				int64(abi.RegisteredSealProof_StackedDrg64GiBV1),
			},
		},
	}
}
