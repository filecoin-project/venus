package chn

import (
	"context"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/ipfs/go-cid"
	hamt "github.com/ipfs/go-hamt-ipld"
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
func (chn *Chain) Head() types.TipSet {
	ts, _ := chn.reader.GetTipSetAndState(chn.reader.GetHead())
	return ts.TipSet
}

// GetRecentAncestorsOfHeaviestChain returns the recent ancestors of the
// `TipSet` with height `descendantBlockHeight` in the heaviest chain.
func (chn *Chain) GetRecentAncestorsOfHeaviestChain(ctx context.Context, descendantBlockHeight *types.BlockHeight) ([]types.TipSet, error) {
	return chain.GetRecentAncestorsOfHeaviestChain(ctx, chn.reader, descendantBlockHeight)
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
