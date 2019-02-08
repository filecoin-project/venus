package series

import (
	"context"
	"time"

	"github.com/filecoin-project/go-filecoin/protocol/storage"
	"github.com/filecoin-project/go-filecoin/tools/fast"
)

// WaitForDealState will query the storage deal until its state matches the
// passed in `state`, or the context is canceled.
func WaitForDealState(ctx context.Context, client *fast.Filecoin, deal *storage.DealResponse, state storage.DealState) error {
	for {
		// Client waits around for the deal to be sealed
		dr, err := client.ClientQueryStorageDeal(ctx, deal.ProposalCid)
		if err != nil {
			return err
		}

		if dr.State == state {
			break
		}

		time.Sleep(time.Second * 30)
	}

	return nil
}
