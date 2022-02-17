package market_event_api

import (
	"context"
	"github.com/filecoin-project/venus/venus-shared/types/gateway"
)

type IMarketEventAPI interface {
	ResponseMarketEvent(ctx context.Context, resp *gateway.ResponseEvent) error                                        //perm:write
	ListenMarketEvent(ctx context.Context, policy *gateway.MarketRegisterPolicy) (<-chan *gateway.RequestEvent, error) //perm:write
}
