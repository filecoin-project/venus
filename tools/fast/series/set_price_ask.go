package series

import (
	"context"
	"fmt"
	"math/big"

	"github.com/filecoin-project/venus/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/venus/tools/fast"
)

// SetPriceGetAsk issues a `set-price` and tries, to the best it can, return the
// created ask. This series will run until it finds an ask, or the context is
// canceled.
func SetPriceGetAsk(ctx context.Context, miner *fast.Filecoin, price *big.Float, expiry *big.Int) (porcelain.Ask, error) {
	// Set a price
	_, err := miner.MinerSetPrice(ctx, price, expiry, fast.AOPrice(big.NewFloat(1.0)), fast.AOLimit(300))
	if err != nil {
		return porcelain.Ask{}, err
	}

	// Dragons: must be re-integrated with storage market module
	return porcelain.Ask{}, fmt.Errorf("could not find ask")
}
