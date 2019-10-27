package submodule

import (
	"context"

	bserv "github.com/ipfs/go-blockservice"
)

// BlockServiceSubmodule enhances the `Node` with networked key/value fetching capabilities.
//
// TODO: split chain data from piece data (issue: https://github.com/filecoin-project/go-filecoin/issues/3481)
// Note: at present:
// - `BlockService` is shared by chain/graphsync and piece/bitswap data
type BlockServiceSubmodule struct {
	// Blockservice is a higher level interface for fetching data
	Blockservice bserv.BlockService
}

// NewBlockserviceSubmodule creates a new block service submodule.
func NewBlockserviceSubmodule(ctx context.Context, blockstore *BlockstoreSubmodule, network *NetworkSubmodule) (BlockServiceSubmodule, error) {
	bservice := bserv.New(blockstore.Blockstore, network.Bitswap)

	return BlockServiceSubmodule{
		Blockservice: bservice,
	}, nil
}
