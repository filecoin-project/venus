package v1

import (
	"context"

	"github.com/filecoin-project/venus/venus-shared/types"
)

type IMarket interface {
	StateMarketParticipants(ctx context.Context, tsk types.TipSetKey) (map[string]types.MarketBalance, error) //perm:read
}
