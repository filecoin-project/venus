package blockservice

import (
	"context"
	"github.com/filecoin-project/venus/app/submodule/apiface"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	"io"
)

var _ apiface.IBlockService = &blockServiceAPI{}

type blockServiceAPI struct { //nolint
	blockService *BlockServiceSubmodule
}

// DAGGetNode returns the associated DAG node for the passed in CID.
func (blockServiceAPI *blockServiceAPI) DAGGetNode(ctx context.Context, ref string) (interface{}, error) {
	return blockServiceAPI.blockService.Dag.GetNode(ctx, ref)
}

// DAGGetFileSize returns the file size for a given Cid
func (blockServiceAPI *blockServiceAPI) DAGGetFileSize(ctx context.Context, c cid.Cid) (uint64, error) {
	return blockServiceAPI.blockService.Dag.GetFileSize(ctx, c)
}

// DAGCat returns an iostream with a piece of data stored on the merkeldag with
// the given cid.
func (blockServiceAPI *blockServiceAPI) DAGCat(ctx context.Context, c cid.Cid) (io.Reader, error) {
	return blockServiceAPI.blockService.Dag.Cat(ctx, c)
}

// DAGImportData adds data from an io reader to the merkledag and returns the
// Cid of the given data. Once the data is in the DAG, it can fetched from the
// node via Bitswap and a copy will be kept in the blockstore.
func (blockServiceAPI *blockServiceAPI) DAGImportData(ctx context.Context, data io.Reader) (ipld.Node, error) {
	return blockServiceAPI.blockService.Dag.ImportData(ctx, data)
}
