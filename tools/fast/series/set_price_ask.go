package series

import (
	"context"
	"fmt"
	"io"
	"math/big"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/tools/fast"
)

// SetPriceGetAsk issues a `set-price` and tries, to the best it can, return the
// created ask. This series will run until it finds an ask, or the context is
// canceled.
func SetPriceGetAsk(ctx context.Context, miner *fast.Filecoin, price *big.Float, expiry *big.Int) (porcelain.Ask, error) {
	// Set a price
	pinfo, err := miner.MinerSetPrice(ctx, price, expiry, fast.AOPrice(big.NewFloat(1.0)), fast.AOLimit(300))
	if err != nil {
		return porcelain.Ask{}, err
	}

	response, err := miner.MessageWait(ctx, pinfo.AddAskCid)
	if err != nil {
		return porcelain.Ask{}, err
	}

	bbs := response.Receipt.Return[0]
	value, err := abi.Deserialize(bbs, abi.Integer)
	if err != nil {
		return porcelain.Ask{}, err
	}

	val, ok := value.Val.(*big.Int)
	if !ok {
		return porcelain.Ask{}, fmt.Errorf("could not cast askid")
	}

	var ask porcelain.Ask
	dec, err := miner.ClientListAsks(ctx)
	if err != nil {
		return porcelain.Ask{}, err
	}

	for {
		err := dec.Decode(&ask)
		if err != nil && err != io.EOF {
			return porcelain.Ask{}, err
		}

		if val.Uint64() == ask.ID && ask.Miner == pinfo.MinerAddr {
			return ask, nil
		}

		if err == io.EOF {
			break
		}
	}

	return porcelain.Ask{}, fmt.Errorf("could not find ask")
}
