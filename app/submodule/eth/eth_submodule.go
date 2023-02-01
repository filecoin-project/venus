package eth

import (
	"context"
	"fmt"

	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/app/submodule/mpool"
	"github.com/filecoin-project/venus/pkg/config"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
)

func NewEthSubModule(cfg *config.Config,
	chainModule *chain.ChainSubmodule,
	mpoolModule *mpool.MessagePoolSubmodule,
	sqlitePath string,
) (*EthSubModule, error) {
	em := &EthSubModule{
		cfg:         cfg,
		chainModule: chainModule,
		mpoolModule: mpoolModule,
		sqlitePath:  sqlitePath,
	}
	ee, err := newEthEventAPI(em)
	if err != nil {
		return nil, fmt.Errorf("create eth event api error %v", err)
	}
	em.ethEventAPI = ee

	em.ethAPIAdapter = &ethAPIDummy{}
	if em.cfg.FevmConfig.EnableEthRPC {
		em.ethAPIAdapter, err = newEthAPI(em)
		if err != nil {
			return nil, err
		}
	}

	return em, nil
}

type EthSubModule struct { // nolint
	cfg         *config.Config
	chainModule *chain.ChainSubmodule
	mpoolModule *mpool.MessagePoolSubmodule
	sqlitePath  string

	ethEventAPI   *ethEventAPI
	ethAPIAdapter ethAPIAdapter
}

func (em *EthSubModule) Start(ctx context.Context) error {
	if err := em.ethEventAPI.Start(ctx); err != nil {
		return err
	}

	return em.ethAPIAdapter.start(ctx)
}

func (em *EthSubModule) Close(ctx context.Context) error {
	if err := em.ethEventAPI.Close(ctx); err != nil {
		return err
	}

	return em.ethAPIAdapter.close()
}

type ethAPIAdapter interface {
	v1api.IETH
	start(ctx context.Context) error
	close() error
}

type fullETHAPI struct {
	v1api.IETH
	*ethEventAPI
}

var _ v1api.IETH = (*fullETHAPI)(nil)

func (em *EthSubModule) API() v1api.FullETH {
	return &fullETHAPI{
		IETH:        em.ethAPIAdapter,
		ethEventAPI: em.ethEventAPI,
	}
}
