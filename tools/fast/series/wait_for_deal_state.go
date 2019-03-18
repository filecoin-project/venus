package series

import (
	"context"

	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	"github.com/filecoin-project/go-filecoin/tools/fast"
)

// WaitForDealState will query the storage deal until its state matches the
// passed in `state`, or the context is canceled.
func WaitForDealState(ctx context.Context, client *fast.Filecoin, deal *storagedeal.Response, state storagedeal.State) error {
	for {
		// Client waits around for the deal to be sealed
		dr, err := client.ClientQueryStorageDeal(ctx, deal.ProposalCid)
		if err != nil {
			return err
		}

		if dr.State == state {
			break
		}

		SleepDelay()
	}

	return nil
}
