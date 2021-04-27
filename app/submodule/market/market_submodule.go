package market

import (
	"github.com/filecoin-project/venus/app/submodule/apiface"
	"github.com/filecoin-project/venus/pkg/statemanger"
)

// MarketSubmodule enhances the `Node` with market capabilities.
type MarketSubmodule struct { //nolint
	c  apiface.IChain
	sm statemanger.IStateManager
}

// NewMarketModule create new market module
func NewMarketModule(c apiface.IChain, sm statemanger.IStateManager) *MarketSubmodule { //nolint
	return &MarketSubmodule{c, sm}
}

func (ms *MarketSubmodule) API() apiface.IMarket {
	return newMarketAPI(ms.c, ms.sm)
}
