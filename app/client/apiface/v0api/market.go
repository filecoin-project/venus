package v0api

import (
	"context"

	apitypes "github.com/filecoin-project/venus/venus-shared/api/chain"
	types "github.com/filecoin-project/venus/venus-shared/chain"
)

type IMarket interface {
	// Rule[perm:read]
	StateMarketParticipants(ctx context.Context, tsk types.TipSetKey) (map[string]apitypes.MarketBalance, error) //perm:admin
}
