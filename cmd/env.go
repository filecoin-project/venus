package cmd

import (
	"context"

	cmds "gx/ipfs/Qmc5paX4ECBARnAKkcAmUYHBGor228Tkfxeya3Nu2KRL46/go-ipfs-cmds"

	"github.com/filecoin-project/go-filecoin/node"
)

// Env is the environment passed to commands.
type Env struct {
	ctx  context.Context
	Node *node.Node
}

// Context returns the context of the environment.
func (ce *Env) Context() context.Context {
	return ce.ctx
}

// GetNode returns the Filecoin node of the environment.
func GetNode(env cmds.Environment) *node.Node {
	ce := env.(*Env)
	return ce.Node
}
