package chn

import (
	"context"

	"github.com/filecoin-project/go-filecoin/consensus"
)

// ChainReader defines a source of block history.
type ChainReader interface {
	BlockHistory(ctx context.Context) <-chan interface{}
	Head() consensus.TipSet
}

// Lser is plumbing implementation for inspecting the blockchain
type Lser struct {
	chainReader ChainReader
}

// New returns a new Lser.
func New(chainReader ChainReader) *Lser {
	return &Lser{chainReader: chainReader}
}

// Ls returns a channel historical tip sets from head to genesis
// If an error is encountered while reading the chain, the error is sent, and the channel is closed.
func (c *Lser) Ls(ctx context.Context) <-chan interface{} {
	return c.chainReader.BlockHistory(ctx)
}

// Head returns the head tipset of the chain
func (c *Lser) Head(ctx context.Context) consensus.TipSet {
	return c.chainReader.Head()
}
