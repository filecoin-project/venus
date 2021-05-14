package market

import (
	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/pkg/statemanger"
)

type MarketSubmodule struct { //nolint
	c  chain.IChain
	sm statemanger.IStateManager
}

func NewMarketModule(c chain.IChain, sm statemanger.IStateManager) *MarketSubmodule { //nolint
	return &MarketSubmodule{c, sm}
}
func (ms *MarketSubmodule) API() IMarket {
	return newMarketAPI(ms.c, ms.sm)
}
