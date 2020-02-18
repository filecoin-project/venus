package series

import (
	"context"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
	storageimpl "github.com/filecoin-project/go-fil-markets/storagemarket/impl"

	"github.com/filecoin-project/go-filecoin/tools/fast"
)

// WaitForDealState will query the storage deal until its state matches the
// passed in `state`, or the context is canceled.
func WaitForDealState(ctx context.Context, client *fast.Filecoin, deal *storageimpl.Response, state storagemarket.StorageDealStatus) (*storageimpl.Response, error) {
	for {
		// Client waits around for the deal to be sealed
		dr, err := client.ClientQueryStorageDeal(ctx, deal.Proposal)
		if err != nil {
			return nil, err
		}

		if dr.State == state {
			return dr, nil
		}

		<-CtxSleepDelay(ctx)
	}
}
