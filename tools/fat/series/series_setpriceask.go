package series

import (
	"context"
	"io"
	"math/big"
	"time"

	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/tools/fat"
)

// SetPriceGetAsk issues a `set-price` and tries, to the best it can, return the
// created ask. This series will run until it finds an ask, or the context is
// canceled.
func SetPriceGetAsk(ctx context.Context, miner *fast.Filecoin, price *big.Float, expiry *big.Int) (api.Ask, error) {
	// Set a price
	pinfo, err := miner.MinerSetPrice(ctx, price, expiry, fast.AOPrice(big.NewFloat(1.0)), fast.AOLimit(300))
	if err != nil {
		return api.Ask{}, err
	}

	// Use the information about the ask to find it
	var ask api.Ask
	found := false
	eof := false
	for {
		// Client lists all of the asks
		asks, err := miner.ClientListAsks(ctx)
		if err != nil {
			return api.Ask{}, err
		}

		// Look for the ask we want
		eof = false
		for {
			if err := asks.Decode(&ask); err != nil {
				if err == io.EOF {
					eof = true
				} else {
					return api.Ask{}, err
				}
			}

			if ask.Miner == pinfo.MinerAddr && ask.Price.Equal(pinfo.Price) {
				found = true
				break
			}

			if eof {
				break
			}
		}

		if found {
			break
		}

		time.Sleep(time.Second * 30)
	}

	return ask, nil
}
