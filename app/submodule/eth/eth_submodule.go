package eth

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/app/submodule/mpool"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/ethhashlookup"
	"github.com/filecoin-project/venus/pkg/events"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
)

func NewEthSubModule(cfg *config.Config,
	chainModule *chain.ChainSubmodule,
	mpoolModule *mpool.MessagePoolSubmodule,
	txHashDBPath string,
) (*EthSubModule, error) {
	em := &EthSubModule{
		cfg:          cfg,
		chainModule:  chainModule,
		mpoolModule:  mpoolModule,
		txHashDBPath: txHashDBPath,
	}
	ee, err := newEthEventAPI(em)
	if err != nil {
		return nil, fmt.Errorf("create eth event api error %v", err)
	}
	em.ethEventAPI = ee

	return em, nil
}

type EthSubModule struct { // nolint
	cfg          *config.Config
	chainModule  *chain.ChainSubmodule
	mpoolModule  *mpool.MessagePoolSubmodule
	txHashDBPath string

	ethEventAPI      *ethEventAPI
	ethTxHashManager *ethTxHashManager // may nil
}

func (em *EthSubModule) Start(ctx context.Context) error {
	if err := em.ethEventAPI.Start(ctx); err != nil {
		return err
	}

	if len(em.txHashDBPath) == 0 {
		return nil
	}

	transactionHashLookup, err := ethhashlookup.NewTransactionHashLookup(filepath.Join(em.txHashDBPath, "txhash.db"))
	if err != nil {
		return err
	}

	ethTxHashManager := ethTxHashManager{
		chainAPI:              em.chainModule.API(),
		TransactionHashLookup: transactionHashLookup,
	}
	em.ethTxHashManager = &ethTxHashManager

	const ChainHeadConfidence = 1
	ev, err := events.NewEventsWithConfidence(ctx, em.ethTxHashManager.chainAPI, ChainHeadConfidence)
	if err != nil {
		return err
	}

	// Tipset listener
	_ = ev.Observe(&ethTxHashManager)

	ch, err := em.mpoolModule.MPool.Updates(ctx)
	if err != nil {
		return err
	}
	go waitForMpoolUpdates(ctx, ch, &ethTxHashManager)
	go ethTxHashGC(ctx, em.cfg.FevmConfig.EthTxHashMappingLifetimeDays, &ethTxHashManager)

	return nil

}

func (em *EthSubModule) Close(ctx context.Context) error {
	if err := em.ethEventAPI.Close(ctx); err != nil {
		return err
	}
	if em.ethTxHashManager != nil {
		return em.ethTxHashManager.TransactionHashLookup.Close()
	}

	return nil
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
