package commands

import (
	"context"

	"github.com/ipfs/go-ipfs-cmds"

	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/protocol/block"
	"github.com/filecoin-project/go-filecoin/protocol/retrieval"
	"github.com/filecoin-project/go-filecoin/protocol/storage"
)

// Env is the environment passed to commands. Implements cmds.Environment.
type Env struct {
	blockMiningAPI *block.MiningAPI
	ctx            context.Context
	porcelainAPI   *porcelain.API
	retrievalAPI   *retrieval.API
	storageAPI     *storage.API
	inspectorAPI   *Inspector
}

var _ cmds.Environment = (*Env)(nil)

// Context returns the context of the environment.
func (ce *Env) Context() context.Context {
	return ce.ctx
}

// GetPorcelainAPI returns the porcelain.API interface from the environment.
func GetPorcelainAPI(env cmds.Environment) *porcelain.API {
	ce := env.(*Env)
	return ce.porcelainAPI
}

// GetBlockAPI returns the block protocol api from the given environment.
func GetBlockAPI(env cmds.Environment) *block.MiningAPI {
	ce := env.(*Env)
	return ce.blockMiningAPI
}

// GetRetrievalAPI returns the retrieval protocol api from the given environment.
func GetRetrievalAPI(env cmds.Environment) *retrieval.API {
	ce := env.(*Env)
	return ce.retrievalAPI
}

// GetStorageAPI returns the storage protocol api from the given environment.
func GetStorageAPI(env cmds.Environment) *storage.API {
	ce := env.(*Env)
	return ce.storageAPI
}

// GetInspectorAPI returns the inspector api from the given environment.
func GetInspectorAPI(env cmds.Environment) *Inspector {
	ce := env.(*Env)
	return ce.inspectorAPI
}
