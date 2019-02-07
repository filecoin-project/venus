package series

import (
	"context"
	"math/big"

	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/tools/fast"
)

// CreateMinerWithAsk setups a miner and sets an ask price. The created ask is
// returned. The node will be mining as well.
func CreateMinerWithAsk(ctx context.Context, miner *fast.Filecoin, pledge uint64, collateral *big.Int, price *big.Float, expiry *big.Int) (api.Ask, error) {

	// Create miner
	_, err := miner.MinerCreate(ctx, pledge, collateral, fast.AOPrice(big.NewFloat(1.0)), fast.AOLimit(300))
	if err != nil {
		return api.Ask{}, err
	}

	if err := miner.MiningStart(ctx); err != nil {
		return api.Ask{}, err
	}

	return SetPriceGetAsk(ctx, miner, price, expiry)

}
