package dagservice

import (
	"context"
	"io"

	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
)

var _ IDagService = &dagServiceAPI{}

type dagServiceAPI struct { //nolint
	dagService *DagServiceSubmodule
}

// DAGGetNode returns the associated DAG node for the passed in CID.
func (dagServiceAPI *dagServiceAPI) DAGGetNode(ctx context.Context, ref string) (interface{}, error) {
	return dagServiceAPI.dagService.Dag.GetNode(ctx, ref)
}

// DAGGetFileSize returns the file size for a given Cid
func (dagServiceAPI *dagServiceAPI) DAGGetFileSize(ctx context.Context, c cid.Cid) (uint64, error) {
	return dagServiceAPI.dagService.Dag.GetFileSize(ctx, c)
}

// DAGCat returns an iostream with a piece of data stored on the merkeldag with
// the given cid.
func (dagServiceAPI *dagServiceAPI) DAGCat(ctx context.Context, c cid.Cid) (io.Reader, error) {
	return dagServiceAPI.dagService.Dag.Cat(ctx, c)
}

// DAGImportData adds data from an io reader to the merkledag and returns the
// Cid of the given data. Once the data is in the DAG, it can fetched from the
// node via Bitswap and a copy will be kept in the blockstore.
func (dagServiceAPI *dagServiceAPI) DAGImportData(ctx context.Context, data io.Reader) (ipld.Node, error) {
	return dagServiceAPI.dagService.Dag.ImportData(ctx, data)
}
