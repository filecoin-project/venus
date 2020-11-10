package series

import (
	"context"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/network"

	"github.com/filecoin-project/venus/tools/fast"
)

// WaitForDealState will query the storage deal until its state matches the
// passed in `state`, or the context is canceled.
func WaitForDealState(ctx context.Context, client *fast.Filecoin, deal *network.Response, state storagemarket.StorageDealStatus) (*network.Response, error) {
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
