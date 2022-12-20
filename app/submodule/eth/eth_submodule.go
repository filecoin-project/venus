package eth

import (
	"context"
	"fmt"

	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/app/submodule/mpool"
	"github.com/filecoin-project/venus/pkg/config"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
)

func NewEthSubModule(actorEventCfg *config.ActorEventConfig,
	chainModule *chain.ChainSubmodule,
	mpoolModule *mpool.MessagePoolSubmodule,
) (*EthSubModule, error) {
	em := &EthSubModule{
		actorEventCfg: actorEventCfg,
		chainModule:   chainModule,
		mpoolModule:   mpoolModule,
	}
	ee, err := newEthEventAPI(em)
	if err != nil {
		return nil, fmt.Errorf("create eth event api error %v", err)
	}
	em.ethEventAPI = ee

	return em, nil
}

type EthSubModule struct { // nolint
	actorEventCfg *config.ActorEventConfig
	chainModule   *chain.ChainSubmodule
	mpoolModule   *mpool.MessagePoolSubmodule

	ethEventAPI *ethEventAPI
}

func (em *EthSubModule) Start(ctx context.Context) error {
	return em.ethEventAPI.Start(ctx)
}

func (em *EthSubModule) Close(ctx context.Context) error {
	return em.ethEventAPI.Close(ctx)
}

type fullETHAPI struct {
	*ethAPI
	*ethEventAPI
}

var _ v1api.IETH = (*fullETHAPI)(nil)

func (em *EthSubModule) API() v1api.IETH {
	return &fullETHAPI{
		ethAPI:      newEthAPI(em),
		ethEventAPI: em.ethEventAPI,
	}
}
