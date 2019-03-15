package net

import (
	"context"
	"fmt"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmWoXtvgC8inqFkAATB7cp2Dax7XBi9VDvSg9RCCZufmRk/go-block-format"
	bserv "gx/ipfs/QmZsGVGCqMCNzHLNMB6q4F6yyvomqf1VxwhJwSfgo1NGaF/go-blockservice"

	"github.com/filecoin-project/go-filecoin/types"
)

// Fetcher is used to fetch data over the network.  It is implemented with
// a persistent bitswap session on a networked blockservice.
type Fetcher struct {
	// session is a bitswap session that enables efficient transfer.
	session *bserv.Session
}

// NewFetcher returns a Fetcher wired up to the input BlockService and a newly
// initialized persistent session of the block service.
func NewFetcher(ctx context.Context, bsrv bserv.BlockService) *Fetcher {
	return &Fetcher{
		session: bserv.NewSession(ctx, bsrv),
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
		blocks = append(blocks, block)
	}
	return blocks, nil
}
