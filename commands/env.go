package commands

import (
	"context"

	cmds "github.com/ipfs/go-ipfs-cmds"

	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/porcelain"
)

// Env is the environment passed to commands. Implements cmds.Environment.
type Env struct {
	ctx          context.Context
	api          api.API
	porcelainAPI *porcelain.API
}

var _ cmds.Environment = (*Env)(nil)

// Context returns the context of the environment.
func (ce *Env) Context() context.Context {
	return ce.ctx
}

// API returns the associated FilecoinAPI object.
func (ce *Env) API() api.API {
	return ce.api
}

// GetAPI returns the Filecoin API object of the environment.
func GetAPI(env cmds.Environment) api.API {
	ce := env.(*Env)
	return ce.API()
}

// GetPorcelainAPI returns the porcelain.API interface from the environment.
func GetPorcelainAPI(env cmds.Environment) *porcelain.API {
	ce := env.(*Env)
	return ce.porcelainAPI
}
