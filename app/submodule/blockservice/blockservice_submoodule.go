package blockservice

import (
	"context"
	"github.com/filecoin-project/venus/app/submodule/apiface"
	"github.com/filecoin-project/venus/app/submodule/blockstore"
	"github.com/filecoin-project/venus/app/submodule/network"
	"github.com/filecoin-project/venus/pkg/util/dag"
	"github.com/ipfs/go-merkledag"

	bserv "github.com/ipfs/go-blockservice"
)

// BlockServiceSubmodule enhances the `Node` with networked key/value fetching capabilities.
//
// TODO: split chain data from piece data (issue: https://github.com/filecoin-project/venus/issues/3481)
// Note: at present:
// - `BlockService` is shared by chain/graphsync and piece/bitswap data
type BlockServiceSubmodule struct { //nolint
	// blockservice is a higher level interface for fetching data
	Blockservice bserv.BlockService

	Dag *dag.DAG
}

// NewBlockserviceSubmodule creates a new block service submodule.
func NewBlockserviceSubmodule(ctx context.Context, blockstore *blockstore.BlockstoreSubmodule, network *network.NetworkSubmodule) (*BlockServiceSubmodule, error) {
	bservice := bserv.New(blockstore.Blockstore, network.Bitswap)
	dag := dag.NewDAG(merkledag.NewDAGService(bservice))
	return &BlockServiceSubmodule{
		Blockservice: bservice,
		Dag:          dag,
	}, nil
}

func (blockService *BlockServiceSubmodule) API() apiface.IBlockService {
	return &blockServiceAPI{blockService: blockService}
}

func (blockService *BlockServiceSubmodule) V0API() apiface.IBlockService {
	return &blockServiceAPI{blockService: blockService}
}
