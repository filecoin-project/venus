package commands

import (
	"context"

	cmds "gx/ipfs/QmUf5GFfV2Be3UtSAPKDVkoRd1TwEBTmx9TSSCFGGjNgdQ/go-ipfs-cmds"

	"github.com/filecoin-project/go-filecoin/node"
)

// Env is the environment passed to commands. Implements cmds.Environment.
type Env struct {
	ctx  context.Context
	node *node.Node
}

var _ cmds.Environment = (*Env)(nil)

// Context returns the context of the environment.
func (ce *Env) Context() context.Context {
	return ce.ctx
}

// Node returns the associated Filecoin node.
func (ce *Env) Node() *node.Node {
	return ce.node
}

// GetNode returns the Filecoin node of the environment.
func GetNode(env cmds.Environment) *node.Node {
	ce := env.(*Env)
	return ce.Node()
}
