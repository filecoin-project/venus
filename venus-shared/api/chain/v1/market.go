package v1

import (
	"context"

	"github.com/filecoin-project/venus/venus-shared/chain"
)

type IMarket interface {
	// Rule[perm:read]
	StateMarketParticipants(ctx context.Context, tsk chain.TipSetKey) (map[string]MarketBalance, error) //perm:admin
}
