package eth

import (
	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/pkg/config"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
)

func NewEthSubModule(chainModule *chain.ChainSubmodule, networkCfg *config.NetworkParamsConfig) (*EthSubModule, error) {
	return &EthSubModule{
		chainModule: chainModule,
		networkCfg:  networkCfg,
	}, nil
}

type EthSubModule struct { // nolint
	chainModule *chain.ChainSubmodule
	networkCfg  *config.NetworkParamsConfig
}

func (em *EthSubModule) API() v1api.IETH {
	return &ethAPI{em: em}
}
