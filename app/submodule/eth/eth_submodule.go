package eth

import (
	"context"
	"fmt"
	"time"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/app/submodule/mpool"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
)

func NewEthSubModule(ctx context.Context,
	cfg *config.Config,
	chainModule *chain.ChainSubmodule,
	mpoolModule *mpool.MessagePoolSubmodule,
	sqlitePath string,
	syncAPI v1api.ISyncer,
) (*EthSubModule, error) {
	ctx, cancel := context.WithCancel(ctx)
	em := &EthSubModule{
		cfg:         cfg,
		chainModule: chainModule,
		mpoolModule: mpoolModule,
		sqlitePath:  sqlitePath,
		ctx:         ctx,
		cancel:      cancel,
		syncAPI:     syncAPI,
	}
	ee, err := newEthEventAPI(ctx, em)
	if err != nil {
		return nil, fmt.Errorf("create eth event api error %v", err)
	}
	em.ethEventAPI = ee

	em.ethAPIAdapter = &ethAPIDummy{}
	if em.cfg.FevmConfig.EnableEthRPC || constants.FevmEnableEthRPC {
		log.Debug("enable eth rpc")
		em.ethAPIAdapter, err = newEthAPI(em)
		if err != nil {
			return nil, err
		}

		// prefill the whole skiplist cache maintained internally by the GetTipsetByHeight
		go func() {
			start := time.Now()
			log.Infoln("Start prefilling GetTipsetByHeight cache")
			_, err := em.chainModule.ChainReader.GetTipSetByHeight(ctx, em.chainModule.ChainReader.GetHead(), abi.ChainEpoch(0), false)
			if err != nil {
				log.Warnf("error when prefilling GetTipsetByHeight cache: %w", err)
			}
			log.Infof("Prefilling GetTipsetByHeight done in %s", time.Since(start))
		}()
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

	ctx     context.Context
	cancel  context.CancelFunc
	syncAPI v1api.ISyncer
}

func (em *EthSubModule) Start(_ context.Context) error {
	if err := em.ethEventAPI.Start(em.ctx); err != nil {
		return err
	}

	return em.ethAPIAdapter.start(em.ctx)
}

func (em *EthSubModule) Close(ctx context.Context) error {
	// exit waitForMpoolUpdates, avoid panic
	em.cancel()

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
