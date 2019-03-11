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
	var tipSetBuffer []types.TipSet

	for raw := range plumbing.ChainLs(ctx) {
		switch v := raw.(type) {
		case error:
			return nil, errors.Wrap(v, "error walking chain")
		case types.TipSet:
			tipSetBuffer = append(tipSetBuffer, v)
		default:
			return nil, errors.New("unexpected type")
		}
	}

	return miner.SampleChainRandomness(sampleHeight, tipSetBuffer)
}
