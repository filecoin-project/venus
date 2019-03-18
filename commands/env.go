package commands

import (
	"context"

	"gx/ipfs/Qmf46mr235gtyxizkKUkTH5fo62Thza2zwXR4DWC7rkoqF/go-ipfs-cmds"

	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/protocol/block"
	"github.com/filecoin-project/go-filecoin/protocol/retrieval"
	"github.com/filecoin-project/go-filecoin/protocol/storage"
)

// Env is the environment passed to commands. Implements cmds.Environment.
type Env struct {
	ctx          context.Context
	api          api.API
	porcelainAPI *porcelain.API
	blockAPI     *block.API
	retrievalAPI *retrieval.API
	storageAPI   *storage.API
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

// GetBlockAPI returns the block protocol api from the given environment
func GetBlockAPI(env cmds.Environment) *block.API {
	ce := env.(*Env)
	return ce.blockAPI
}

// GetRetrievalAPI returns the retrieval protocol api from the given environment
func GetRetrievalAPI(env cmds.Environment) *retrieval.API {
	ce := env.(*Env)
	return ce.retrievalAPI
}

// GetStorageAPI returns the storage protocol api from the given environment
func GetStorageAPI(env cmds.Environment) *storage.API {
	ce := env.(*Env)
	return ce.storageAPI
}
