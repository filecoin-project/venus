package v0

import (
	"context"

	chain2 "github.com/filecoin-project/venus/venus-shared/api/chain"
	"github.com/filecoin-project/venus/venus-shared/chain"
)

type IMarket interface {
	StateMarketParticipants(ctx context.Context, tsk chain.TipSetKey) (map[string]chain2.MarketBalance, error) //perm:read
}
