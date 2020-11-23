package series

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/venus/tools/fast"
)

// WaitForBlockHeight will inspect the chain head and wait till the height is equal to or
// greater than the provide height `bh`
func WaitForBlockHeight(ctx context.Context, client *fast.Filecoin, bh abi.ChainEpoch) error {
	for {

		hh, err := GetHeadBlockHeight(ctx, client)
		if err != nil {
			return err
		}

		if hh >= bh {
			break
		}

		<-CtxSleepDelay(ctx)
	}

	return nil
}
