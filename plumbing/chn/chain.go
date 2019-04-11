package chn

import (
	"context"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/sampling"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/ipfs/go-cid"
	hamt "github.com/ipfs/go-hamt-ipld"
	"github.com/pkg/errors"
)

// Chain is a simple decorator for the chain core api
type Chain struct {
	// To get the head tipset state root.
	reader chain.ReadStore
	// To load the tree for the head tipset state root.
	cst *hamt.CborIpldStore
}

// NewChain returns a new Chain.
func NewChain(chainReader chain.ReadStore, cst *hamt.CborIpldStore) *Chain {
	return &Chain{
		reader: chainReader,
		cst:    cst,
	}
}

// Head returns the head tipset
func (chn *Chain) Head() (*types.TipSet, error) {
	ts, err := chn.reader.GetTipSetAndState(chn.reader.GetHead())
	if err != nil {
		return nil, err
	}
	return &ts.TipSet, nil
}

// Ls returns a channel of tipsets from head to genesis
func (chn *Chain) Ls(ctx context.Context) (*chain.TipsetIterator, error) {
	tsas, err := chn.reader.GetTipSetAndState(chn.reader.GetHead())
	if err != nil {
		return nil, err
	}
	return chain.IterAncestors(ctx, chn.reader, tsas.TipSet), nil
}

// GetBlock gets a block by CID
func (chn *Chain) GetBlock(ctx context.Context, id cid.Cid) (*types.Block, error) {
	return chn.reader.GetBlock(ctx, id)
}

// SampleRandomness samples randomness from the chain at the given height.
func (chn *Chain) SampleRandomness(ctx context.Context, sampleHeight *types.BlockHeight) ([]byte, error) {
	tipSetBuffer, err := chain.GetRecentAncestorsOfHeaviestChain(ctx, chn.reader, sampleHeight)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get recent ancestors")
	}

	return sampling.SampleChainRandomness(sampleHeight, tipSetBuffer)
}
