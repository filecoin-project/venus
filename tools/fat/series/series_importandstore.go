package series

import (
	"context"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmXWZCd8jfaHmt4UDSnjKmGcrQMw95bDGWqEeVLVJjoANX/go-ipfs-files"

	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/protocol/storage"
	"github.com/filecoin-project/go-filecoin/tools/fat"
)

// ImportAndStore imports the `data` to the `client`, and proposes a storage
// deal using the provided `ask`, returning the cid of the import and the
// created deal.
func ImportAndStore(ctx context.Context, client *fast.Filecoin, ask api.Ask, data files.File) (cid.Cid, *storage.DealResponse, error) {
	// Client neeeds to import the data
	dcid, err := client.ClientImport(ctx, data)
	if err != nil {
		return cid.Undef, nil, err
	}

	// Client makes a deal
	deal, err := client.ClientProposeStorageDeal(ctx, dcid, ask.Miner, ask.ID, 10, false)
	if err != nil {
		return cid.Undef, nil, err
	}

	return dcid, deal, nil
}
