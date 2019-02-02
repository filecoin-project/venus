package porcelain

import (
	"context"
	"math/big"
	"time"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/plumbing"
	"github.com/filecoin-project/go-filecoin/types"
)

// API is the porcelain implementation, a set of convenience calls written on the
// plumbing api, to be used to build user facing features and protocols. Note that porcelain.API
// embeds the plumbing.API, so plumbing calls are available directly through this struct. That
// way consumers only have to take one dependency and they can selectively mock things out
// at whatever level is appropriate.
//
// It is a feature that API delegates to free functions for implementation. This enables each
// implementation to depend on the specific narrow set of plumbing that it uses.
type API struct {
	*plumbing.API
}

// New returns a new porcelain.API.
func New(plumbing *plumbing.API) *API {
	return &API{plumbing}
}

// MessageSendWithRetry sends a message and retries if it does not appear on chain. See implementation
// for more details.
func (a *API) MessageSendWithRetry(ctx context.Context, numRetries uint, waitDuration time.Duration, from, to address.Address, val *types.AttoFIL, method string, gasPrice types.AttoFIL, gasLimit types.GasUnits, params ...interface{}) (err error) {
	return MessageSendWithRetry(ctx, a, numRetries, waitDuration, from, to, val, method, gasPrice, gasLimit, params...)
}

// MinerSetPrice configures the price of storage. See implementation for details.
func (a *API) MinerSetPrice(ctx context.Context, from address.Address, miner address.Address, gasPrice types.AttoFIL, gasLimit types.GasUnits, price *types.AttoFIL, expiry *big.Int) (MinerSetPriceResponse, error) {
	return MinerSetPrice(ctx, a, from, miner, gasPrice, gasLimit, price, expiry)
}
