package commands

import (
	"context"

	cmds "gx/ipfs/Qma6uuSyjkecGhMFFLfzyJDPyoDtNJSHJNweDccZhaWkgU/go-ipfs-cmds"

	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/api2"
)

// Env is the environment passed to commands. Implements cmds.Environment.
type Env struct {
	ctx  context.Context
	api  api.API
	API2 api2.Filecoin
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

// GetAPI2 returns the api2.Filecoin interface from the environment.
func GetAPI2(env cmds.Environment) api2.Filecoin {
	ce := env.(*Env)
	return ce.API2
}
