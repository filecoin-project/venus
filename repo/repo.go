package repo

import (
	"github.com/filecoin-project/go-filecoin/config"
	"gx/ipfs/QmPpegoMqhAEqjncrzArm7KVWAkCm78rqL2DPuNjhPrshg/go-datastore"
)

// Datastore is the datastore interface provided by the repo
type Datastore interface {
	// NB: there are other more featureful interfaces we could require here, we
	// can either force it, or just do hopeful type checks. Not all datastores
	// implement every feature.
	datastore.Batching
}

// Repo is a representation of all persistent data in a filecoin node.
type Repo interface {
	Config() *config.Config
	Datastore() Datastore
}
