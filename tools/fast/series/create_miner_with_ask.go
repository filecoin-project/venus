package series

import (
	"context"
	"math/big"

	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/tools/fast"
)

// CreateStorageMinerWithAsk setups a miner and sets an ask price. The created ask is
// returned. The node will be mining as well.
func CreateStorageMinerWithAsk(ctx context.Context, miner *fast.Filecoin, collateral *big.Int, price *big.Float, expiry *big.Int) (porcelain.Ask, error) {

	// Create miner
	_, err := miner.MinerCreate(ctx, collateral, fast.AOPrice(big.NewFloat(1.0)), fast.AOLimit(300))
	if err != nil {
		return porcelain.Ask{}, err
	}

	if err := miner.MiningStart(ctx); err != nil {
		return porcelain.Ask{}, err
	}

	return SetPriceGetAsk(ctx, miner, price, expiry)

}
