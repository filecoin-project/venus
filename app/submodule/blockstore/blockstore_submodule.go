package blockstore

import (
	"context"
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
)

// BlockstoreSubmodule enhances the `Node` with local key/value storing capabilities.
//
// TODO: split chain data from piece data (issue: https://github.com/filecoin-project/venus/issues/3481)
// Note: at present:
// - `Blockstore` is shared by chain/graphsync and piece/bitswap data
// - `cborStore` is used for chain state and shared with piece data exchange for deals at the moment.
type BlockstoreSubmodule struct { //nolint
	// Blockstore is the un-networked blocks interface
	Blockstore bstore.Blockstore

	// cborStore is a wrapper for a `cbor.IpldStore` that works on the local IPLD-Cbor objects stored in `Blockstore`.
	CborStore cbor.IpldStore
}

type blockstoreRepo interface {
	Datastore() blockstoreutil.Blockstore
}

// NewBlockstoreSubmodule creates a new block store submodule.
func NewBlockstoreSubmodule(ctx context.Context, repo blockstoreRepo) (*BlockstoreSubmodule, error) {
	// set up block store
	//bs := bstore.NewBlockstore(repo.Datastore())
	bs := repo.Datastore()
	// setup a ipldCbor on top of the local store
	ipldCborStore := cbor.NewCborStore(bs)

	return &BlockstoreSubmodule{
		Blockstore: bs,
		CborStore:  ipldCborStore,
	}, nil
}

func (bsm *BlockstoreSubmodule) API() *BlockstoreAPI {
	return &BlockstoreAPI{blockstore: bsm}
}
