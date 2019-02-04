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
// plumbing api, to be used to build user facing features and protocols.
//
// The porcelain.API provides porcelain calls **as well as the plumbing calls**.
// This is because most consumers depend on a combination of porcelain and plumbing
// calls. Flattening both apis into a single implementation enables consumers to take
// a single dependency and not have to know which api a call comes from. The mechanism
// is embedding: the plumbing implementation is embedded in the porcelain implementation, making
// all the embedded type (plumbing) calls available on the embedder type (porcelain).
// Providing a single implementation on which to depend also enables consumers to choose
// at what level to mock out their dependencies: low (plumbing) or high (porcelain).
// We ensure that porcelain calls only depend on the narrow subset of the plumbing api
// on which they depend by implementing them in free functions that take their specific
// subset of the plumbing.api. The porcelain.API delegates porcelain calls to these
// free functions.
//
// If you are implementing a user facing feature or a protocol this is probably the implementation
// you should depend on. Define the subset of it that you use in an interface in your package
// take this implementation as a dependency.
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
