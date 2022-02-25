package market

import (
	"github.com/filecoin-project/venus/pkg/statemanger"
	v0api "github.com/filecoin-project/venus/venus-shared/api/chain/v0"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
)

// MarketSubmodule enhances the `Node` with market capabilities.
type MarketSubmodule struct { //nolint
	c  v1api.IChain
	sm statemanger.IStateManager
}

// NewMarketModule create new market module
func NewMarketModule(c v1api.IChain, sm statemanger.IStateManager) *MarketSubmodule { //nolint
	return &MarketSubmodule{c, sm}
}

func (ms *MarketSubmodule) API() v1api.IMarket {
	return newMarketAPI(ms.c, ms.sm)
}

func (ms *MarketSubmodule) V0API() v0api.IMarket {
	return newMarketAPI(ms.c, ms.sm)
}
