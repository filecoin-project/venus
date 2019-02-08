package chn

import (
	"context"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/types"
)

// ChainReader defines a source of block history.
type ChainReader interface {
	BlockHistory(ctx context.Context) <-chan interface{}
	GetBlock(ctx context.Context, id cid.Cid) (*types.Block, error)
}

// Reader is plumbing implementation for inspecting the blockchain
// NOTE: this wrapper is unnecessary and slated for removal.
type Reader struct {
	chainReader ChainReader
}

// New returns a new Reader.
func New(chainReader ChainReader) *Reader {
	return &Reader{chainReader: chainReader}
}

// Ls returns a channel historical tip sets from head to genesis
// If an error is encountered while reading the chain, the error is sent, and the channel is closed.
func (c *Reader) Ls(ctx context.Context) <-chan interface{} {
	return c.chainReader.BlockHistory(ctx)
}

// BlockGet returns a block by its CID
func (c *Reader) BlockGet(ctx context.Context, id cid.Cid) (*types.Block, error) {
	return c.chainReader.GetBlock(ctx, id)
}
