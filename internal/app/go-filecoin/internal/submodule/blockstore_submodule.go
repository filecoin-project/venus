package submodule

import (
	"context"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"
)

// BlockstoreSubmodule enhances the `Node` with local key/value storing capabilities.
//
// TODO: split chain data from piece data (issue: https://github.com/filecoin-project/go-filecoin/issues/3481)
// Note: at present:
// - `Blockstore` is shared by chain/graphsync and piece/bitswap data
// - `cborStore` is used for chain state and shared with piece data exchange for deals at the moment.
type BlockstoreSubmodule struct {
	// Blockstore is the un-networked blocks interface
	Blockstore bstore.Blockstore

	// cborStore is a wrapper for a `hamt.CborIpldStore` that works on the local IPLD-Cbor objects stored in `Blockstore`.
	CborStore hamt.CborIpldStore
}

type blockstoreRepo interface {
	Datastore() ds.Batching
}

// NewBlockstoreSubmodule creates a new block store submodule.
func NewBlockstoreSubmodule(ctx context.Context, repo blockstoreRepo) (BlockstoreSubmodule, error) {
	// set up block store
	bs := bstore.NewBlockstore(repo.Datastore())
	// setup a ipldCbor on top of the local store
	ipldCborStore := hamt.CSTFromBstore(bs)

	return BlockstoreSubmodule{
		Blockstore: bs,
		CborStore:  ipldCborStore,
	}, nil
}
