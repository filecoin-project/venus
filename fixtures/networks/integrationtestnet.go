package networks

import (
	"github.com/filecoin-project/venus/internal/pkg/config"
)

func IntegrationNet() *NetworkConf {

	return &NetworkConf{
		Bootstrap: config.BootstrapConfig{
			Addresses:        []string{},
			MinPeerThreshold: 0,
			Period:           "30s",
		},
		Network: config.NetworkParamsConfig{},
	}
}
