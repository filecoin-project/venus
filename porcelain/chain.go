package porcelain

import (
	"context"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/types"
)

type chPlumbing interface {
	ChainLs(ctx context.Context) <-chan interface{}
}

// ChainBlockHeight determines the current block height
func ChainBlockHeight(ctx context.Context, plumbing chPlumbing) (*types.BlockHeight, error) {
	lsCtx, cancelLs := context.WithCancel(ctx)
	tipSetCh := plumbing.ChainLs(lsCtx)
	head := <-tipSetCh
	cancelLs()

	if head == nil {
		return nil, errors.New("could not retrieve block height")
	}

	currentHeight, err := head.(types.TipSet).Height()
	if err != nil {
		return nil, err
	}
	return types.NewBlockHeight(currentHeight), nil
}

// SampleChainRandomness samples randomness from the chain at the given height.
func SampleChainRandomness(ctx context.Context, plumbing chPlumbing, sampleHeight *types.BlockHeight) ([]byte, error) {
	return miner.SampleChainRandomness(sampleHeight, plumbing.ChainLs(ctx))
}
