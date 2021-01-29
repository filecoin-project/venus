package market

import (
	"github.com/filecoin-project/venus/app/submodule/mpool"
	"github.com/filecoin-project/venus/pkg/market"
)

type MarketSubmodule struct {//nolint
	*mpool.MessagePoolAPI
	fmgr *market.FundManager
}

func NewMarketModule(mpapi *mpool.MessagePoolAPI, params *market.FundManagerParams) *MarketSubmodule {//nolint
	fmgr := market.NewFundManager(params)
	return &MarketSubmodule{mpapi, fmgr}
}
func (mm *MarketSubmodule) API() Market {
	return newMarketAPI(mm.MessagePoolAPI, mm.fmgr)
}

func (mm *MarketSubmodule) Start() error {
	return mm.fmgr.Start()
}

func (mm *MarketSubmodule) Stop() {
	mm.fmgr.Stop()
}
