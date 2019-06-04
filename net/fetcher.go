package net

import (
	"context"
	"fmt"

	"github.com/ipfs/go-block-format"
	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/types"
)

// Fetcher is used to fetch data over the network.  It is implemented with
// a persistent bitswap session on a networked blockservice.
type Fetcher struct {
	// session is a bitswap session that enables efficient transfer.
	session   *bserv.Session
	validator consensus.BlockSyntaxValidator
}

// NewFetcher returns a Fetcher wired up to the input BlockService and a newly
// initialized persistent session of the block service.
func NewFetcher(ctx context.Context, bsrv bserv.BlockService, bv consensus.BlockSyntaxValidator) *Fetcher {
	return &Fetcher{
		session:   bserv.NewSession(ctx, bsrv),
		validator: bv,
	}
}

// GetBlocks fetches the blocks with the given cids from the network using the
// Fetcher's bitswap session.
func (f *Fetcher) GetBlocks(ctx context.Context, cids []cid.Cid) ([]*types.Block, error) {
	var unsanitized []blocks.Block
	for b := range f.session.GetBlocks(ctx, cids) {
		unsanitized = append(unsanitized, b)
	}

	if len(unsanitized) < len(cids) {
		var err error
		if ctxErr := ctx.Err(); ctxErr != nil {
			err = errors.Wrap(ctxErr, "failed to fetch all requested blocks")
		} else {
			err = errors.New("failed to fetch all requested blocks")
		}
		return nil, err
	}

	var blocks []*types.Block
	for _, u := range unsanitized {
		block, err := types.DecodeBlock(u.RawData())
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("fetched data (cid %s) was not a block", u.Cid().String()))
		}

		// reject blocks that are syntactically invalid.
		if err := f.validator.ValidateSyntax(ctx, block); err != nil {
			continue
		}

		blocks = append(blocks, block)
	}
	return blocks, nil
}
