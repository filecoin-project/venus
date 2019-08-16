package series

import (
	"context"
	"math/big"
	"time"

	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/types"
)

// CreateStorageMinerWithAsk setups a miner and sets an ask price. The created ask is
// returned. The node will be mining as well.
func CreateStorageMinerWithAsk(ctx context.Context, miner *fast.Filecoin, collateral *big.Int, price *big.Float, expiry *big.Int, sectorSize *types.BytesAmount) (porcelain.Ask, error) {

	go func() {
		// give miner create enough time to get the message in the queue
		time.Sleep(200 * time.Millisecond)
		CtxMiningOnce(ctx)
	}()

	// Create miner
	_, err := miner.MinerCreate(ctx, collateral, fast.AOSectorSize(sectorSize), fast.AOPrice(big.NewFloat(1.0)), fast.AOLimit(300))
	if err != nil {
		return porcelain.Ask{}, err
	}

	return SetPriceGetAsk(ctx, miner, price, expiry)
}
