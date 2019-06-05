package series

import (
	"context"

	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/types"
)

// WaitForBlockHeight will inspect the chain head and wait till the height is equal to or
// greater than the provide height `bh`
func WaitForBlockHeight(ctx context.Context, client *fast.Filecoin, bh *types.BlockHeight) error {
	for {

		hh, err := GetHeadBlockHeight(ctx, client)
		if err != nil {
			return err
		}

		if hh.GreaterEqual(bh) {
			break
		}

		CtxSleepDelay(ctx)
	}

	return nil
}
