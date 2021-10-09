package networks

import "github.com/filecoin-project/venus/pkg/config"

type NetworkConf struct {
	Bootstrap config.BootstrapConfig
	Network   config.NetworkParamsConfig
}
