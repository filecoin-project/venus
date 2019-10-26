package series

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-filecoin/cmd/go-filecoin"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/ipfs/go-cid"
)

// FindDealByID looks for a deal using `DealsList` and returns the result where id matches the ProposalCid of
// the deal.
func FindDealByID(ctx context.Context, client *fast.Filecoin, id cid.Cid) (commands.DealsListResult, error) {
	dec, err := client.DealsList(ctx)
	if err != nil {
		return commands.DealsListResult{}, err
	}

	var dl commands.DealsListResult

	for dec.More() {
		if err := dec.Decode(&dl); err != nil {
			return commands.DealsListResult{}, err
		}

		if dl.ProposalCid == id {
			return dl, nil
		}
	}

	return commands.DealsListResult{}, fmt.Errorf("No deal found")
}
