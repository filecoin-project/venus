package porcelain

import (
	"context"

	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/sampling"
	"github.com/filecoin-project/go-filecoin/types"
)

type chBlockHeightPlumbing interface {
	ChainHead() types.TipSet
}

type chSampleRandomnessPlumbing interface {
	GetRecentAncestorsOfHeaviestChain(ctx context.Context, descendantBlockHeight *types.BlockHeight) ([]types.TipSet, error)
}

// ChainBlockHeight determines the current block height
func ChainBlockHeight(plumbing chBlockHeightPlumbing) (*types.BlockHeight, error) {
	currentHeight, err := plumbing.ChainHead().Height()
	if err != nil {
		return nil, err
	}
	return types.NewBlockHeight(currentHeight), nil
}

// SampleChainRandomness samples randomness from the chain at the given height.
func SampleChainRandomness(ctx context.Context, plumbing chSampleRandomnessPlumbing, sampleHeight *types.BlockHeight) ([]byte, error) {
	tipSetBuffer, err := plumbing.GetRecentAncestorsOfHeaviestChain(ctx, sampleHeight)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get recent ancestors")
	}

	return sampling.SampleChainRandomness(sampleHeight, tipSetBuffer)
}
