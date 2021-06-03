package market

import (
	"github.com/filecoin-project/venus/app/submodule/apiface"
	"github.com/filecoin-project/venus/pkg/statemanger"
)

type MarketSubmodule struct { //nolint
	c  apiface.IChain
	sm statemanger.IStateManager
}

func NewMarketModule(c apiface.IChain, sm statemanger.IStateManager) *MarketSubmodule { //nolint
	return &MarketSubmodule{c, sm}
}

func (ms *MarketSubmodule) API() apiface.IMarket {
	return newMarketAPI(ms.c, ms.sm)
}

func (ms *MarketSubmodule) V0API() apiface.IMarket {
	return newMarketAPI(ms.c, ms.sm)
}
