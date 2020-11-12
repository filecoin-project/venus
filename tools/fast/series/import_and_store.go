package series

import (
	"context"

	"github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"

	"github.com/filecoin-project/go-fil-markets/storagemarket/network"
	"github.com/filecoin-project/venus/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/venus/tools/fast"
)

// ImportAndStore imports the `data` to the `client`, and proposes a storage
// deal using the provided `ask`, returning the cid of the import and the
// created deal. It uses a duration of 10 blocks
func ImportAndStore(ctx context.Context, client *fast.Filecoin, ask porcelain.Ask, data files.File) (cid.Cid, *network.Response, error) {
	return ImportAndStoreWithDuration(ctx, client, ask, 10, data)
}

// ImportAndStoreWithDuration imports the `data` to the `client`, and proposes a storage
// deal using the provided `ask`, returning the cid of the import and the
// created deal, using the provided duration.:
func ImportAndStoreWithDuration(ctx context.Context, client *fast.Filecoin, ask porcelain.Ask, duration uint64, data files.File) (cid.Cid, *network.Response, error) {
	// Client neeeds to import the data
	dcid, err := client.ClientImport(ctx, data)
	if err != nil {
		return cid.Undef, nil, err
	}

	// Client makes a deal
	deal, err := client.ClientProposeStorageDeal(ctx, dcid, ask.Miner, ask.ID, duration)
	if err != nil {
		return cid.Undef, nil, err
	}

	return dcid, deal, nil
}
