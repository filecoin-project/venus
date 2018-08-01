package commands

import (
	"context"

	cmds "gx/ipfs/QmVTmXZC2yE38SDKRihn96LXX6KwBWgzAg8aCDZaMirCHm/go-ipfs-cmds"

	"github.com/filecoin-project/go-filecoin/core/node"
	"github.com/filecoin-project/go-filecoin/node/iface"
)

// Env is the environment passed to commands. Implements cmds.Environment.
type Env struct {
	ctx context.Context
	api iface.CoreAPI
}

var _ cmds.Environment = (*Env)(nil)

// Context returns the context of the environment.
func (ce *Env) Context() context.Context {
	return ce.ctx
}

// API returns the associated FilecoinAPI object.
func (ce *Env) API() iface.CoreAPI {
	return ce.api
}

// GetAPI returns the Filecoin API object of the environment.
func GetAPI(env cmds.Environment) iface.CoreAPI {
	ce := env.(*Env)
	return ce.API()
}

// Node returns the associated node.
// DEPRECATED
// TODO: remove once all commands are using `API()`
func (ce *Env) Node() *node.Node {
	if ce.api == nil {
		return nil
	}
	return ce.api.Node()
}

// GetNode returns the Filecoin node of the environment.
// DEPRECATED
// TODO: remove once all commands are using `GetAPI()`
func GetNode(env cmds.Environment) *node.Node {
	ce := env.(*Env)
	return ce.Node()
}
