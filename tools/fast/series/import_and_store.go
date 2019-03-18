package series

import (
	"context"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"

	"gx/ipfs/QmQmhotPUzVrMEWNK3x1R5jQ5ZHWyL7tVUrmRPjrBrvyCb/go-ipfs-files"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/tools/fast"
)

// ImportAndStore imports the `data` to the `client`, and proposes a storage
// deal using the provided `ask`, returning the cid of the import and the
// created deal.
func ImportAndStore(ctx context.Context, client *fast.Filecoin, ask porcelain.Ask, data files.File) (cid.Cid, *storagedeal.Response, error) {
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
