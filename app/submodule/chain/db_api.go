package chain

import (
	"context"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/ipfs/go-cid"
	"io"
)

type DbAPI struct {
	chain *ChainSubmodule
}

func NewDbAPI(chain *ChainSubmodule) DbAPI {
	return DbAPI{chain: chain}
}

func (dbAPI *DbAPI) ChainReadObj(ctx context.Context, ocid cid.Cid) ([]byte, error) {
	return dbAPI.chain.State.ReadObj(ctx, ocid)
}

func (dbAPI *DbAPI) ChainHasObj(ctx context.Context, ocid cid.Cid) (bool, error) {
	return dbAPI.chain.State.HasObj(ctx, ocid)
}

// ChainExport exports the chain from `head` up to and including the genesis block to `out`
func (dbAPI *DbAPI) ChainExport(ctx context.Context, head block.TipSetKey, out io.Writer) error {
	return dbAPI.chain.State.ChainExport(ctx, head, out)
}
