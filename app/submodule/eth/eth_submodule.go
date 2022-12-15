package eth

import (
	"github.com/filecoin-project/venus/app/submodule/chain"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
)

func NewEthSubModule(chainModule *chain.ChainSubmodule) (*EthSubModule, error) {
	return &EthSubModule{
		chainModule: chainModule,
	}, nil
}

type EthSubModule struct { // nolint
	chainModule *chain.ChainSubmodule
}

func (em *EthSubModule) API() v1api.IETH {
	return &ethAPI{em: em}
}
