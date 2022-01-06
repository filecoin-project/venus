package apiface

import (
	"context"

	"github.com/filecoin-project/venus/venus-shared/types"
)

type IMarket interface {
	// Rule[perm:read]
	StateMarketParticipants(ctx context.Context, tsk types.TipSetKey) (map[string]types.MarketBalance, error) //perm:admin
}
