package node

import (
	bserv "github.com/ipfs/go-blockservice"
)

// BlockserviceSubmodule enhances the `Node` with networked key/value fetching capabilities.
//
// TODO: split chain data from piece data (issue: https://github.com/filecoin-project/go-filecoin/issues/3481)
// Note: at present:
// - `BlockService` is shared by chain/graphsync and piece/bitswap data
type BlockserviceSubmodule struct {
	// Blockservice is a higher level interface for fetching data
	blockservice bserv.BlockService
}
