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

// SampleChainRandomness produces a slice of random bytes sampled from a TipSet
// in the blockchain at a given height (minus lookback). This is useful for
// things like PoSt challenge seed generation.
//
// If no TipSet exists with the provided height (sampleHeight), we instead
// sample the genesis block.
func SampleChainRandomness(ctx context.Context, plumbing chPlumbing, sampleHeight *types.BlockHeight) ([]byte, error) {
	sampleHeight = sampleHeight.Sub(types.NewBlockHeight(miner.LookbackParameter))
	if sampleHeight.LessThan(types.NewBlockHeight(0)) {
		sampleHeight = types.NewBlockHeight(0)
	}

	var sampleTipSet *types.TipSet

Loop:
	for raw := range plumbing.ChainLs(ctx) {
		switch v := raw.(type) {
		case error:
			return nil, errors.Wrap(v, "error walking chain")
		case types.TipSet:
			height, err := v.Height()
			if err != nil {
				return nil, errors.Wrap(err, "error obtaining tip set height")
			}

			if sampleHeight.Equal(types.NewBlockHeight(height)) {
				sampleTipSet = &v
				break Loop
			}
		default:
			return nil, errors.New("unexpected type")
		}
	}

	if sampleTipSet == nil {
		return nil, errors.Errorf("found no tip set in chain with height %s", sampleHeight)
	}

	ticket, err := sampleTipSet.MinTicket()
	if err != nil {
		return nil, errors.Wrap(err, "error obtaining tip set (min) ticket")
	}

	return ticket, nil
}
