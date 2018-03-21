package repo

import (
	"gx/ipfs/QmPpegoMqhAEqjncrzArm7KVWAkCm78rqL2DPuNjhPrshg/go-datastore"

	"github.com/filecoin-project/go-filecoin/config"
)

// Version is the current repo version that we require for a valid repo.
const Version uint = 1

// Datastore is the datastore interface provided by the repo
type Datastore interface {
	// NB: there are other more featureful interfaces we could require here, we
	// can either force it, or just do hopeful type checks. Not all datastores
	// implement every feature.
	datastore.Batching
	Close() error
}

// Repo is a representation of all persistent data in a filecoin node.
type Repo interface {
	Config() *config.Config
	Datastore() Datastore
	Version() uint
	Close() error
}
