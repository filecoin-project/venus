package commands

import (
	"context"

	cmds "github.com/ipfs/go-ipfs-cmds"

	"github.com/filecoin-project/venus/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/venus/internal/pkg/protocol/drand"
)

// Env is the environment for command API handlers.
type Env struct {
	ctx          context.Context
	drandAPI     *drand.API
	porcelainAPI *porcelain.API
	inspectorAPI *Inspector
}

var _ cmds.Environment = (*Env)(nil)

// NewClientEnv returns a new environment for command API clients.
// This environment lacks direct access to any internal APIs.
func NewClientEnv(ctx context.Context) *Env {
	return &Env{ctx: ctx}
}

// Context returns the context of the environment.
func (ce *Env) Context() context.Context {
	return ce.ctx
}

// GetPorcelainAPI returns the porcelain.API interface from the environment.
func GetPorcelainAPI(env cmds.Environment) *porcelain.API {
	ce := env.(*Env)
	return ce.porcelainAPI
}

//// GetBlockAPI returns the block protocol api from the given environment.
//func GetBlockAPI(env cmds.Environment) *mining.API {
//	ce := env.(*Env)
//	return ce.blockMiningAPI
//}
//
//// GetRetrievalAPI returns the retrieval protocol api from the given environment.
//func GetRetrievalAPI(env cmds.Environment) retrieval.API {
//	ce := env.(*Env)
//	return ce.retrievalAPI
//}
//
//// GetStorageAPI returns the storage protocol api from the given environment.
//func GetStorageAPI(env cmds.Environment) *storage.API {
//	ce := env.(*Env)
//	return ce.storageAPI
//}

// GetInspectorAPI returns the inspector api from the given environment.
func GetInspectorAPI(env cmds.Environment) *Inspector {
	ce := env.(*Env)
	return ce.inspectorAPI
}

// GetDrandAPI returns the drand api from the given environment.
func GetDrandAPI(env cmds.Environment) *drand.API {
	ce := env.(*Env)
	return ce.drandAPI
}
