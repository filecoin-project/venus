package node

import (
	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"
)

// BlockstoreSubmodule enhances the `Node` with key/value storing capabilities.
//
// TODO: split chain data from piece data (issue: https://github.com/filecoin-project/go-filecoin/issues/3481)
// Note: at present:
// - `BlockService` is shared by chain/graphsync and piece/bitswap data
// - `Blockstore` is shared by chain/graphsync and piece/bitswap data
// - `cborStore` is used for chain state and shared with piece data exchange for deals at the moment.
type BlockstoreSubmodule struct {
	// Blockservice is a higher level interface for fetching data
	blockservice bserv.BlockService

	// Blockstore is the un-networked blocks interface
	Blockstore bstore.Blockstore

	// cborStore is a temporary interface for interacting with IPLD objects.
	cborStore *hamt.CborIpldStore
}
