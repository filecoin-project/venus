package chain

import (
	"context"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/ipfs/go-cid"
	"io"
)

func (chainAPI *ChainAPI) ChainReadObj(ctx context.Context, ocid cid.Cid) ([]byte, error) {
	return chainAPI.chain.State.ReadObj(ctx, ocid)
}

func (chainAPI *ChainAPI) ChainHasObj(ctx context.Context, ocid cid.Cid) (bool, error) {
	return chainAPI.chain.State.HasObj(ctx, ocid)
}

// ChainExport exports the chain from `head` up to and including the genesis block to `out`
func (chainAPI *ChainAPI) ChainExport(ctx context.Context, head block.TipSetKey, out io.Writer) error {
	return chainAPI.chain.State.ChainExport(ctx, head, out)
}
