package repo

import (
	"github.com/ipfs/go-datastore"
	ds "github.com/ipfs/go-datastore"
	keystore "github.com/ipfs/go-ipfs-keystore"

	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
)

// Version is the version of repo schema that this code understands.
const Version uint = 2

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
	// ReplaceConfig replaces the current config, with the newly passed in one.
	ReplaceConfig(cfg *config.Config) error

	// Datastore is a general storage solution for things like blocks.
	Datastore() ds.Batching
	Keystore() keystore.Keystore

	// WalletDatastore is a specific storage solution, only used to store sensitive wallet information.
	WalletDatastore() Datastore

	// ChainDatastore is a specific storage solution, only used to store already validated chain data.
	ChainDatastore() Datastore

	// DealsDatastore holds deals data.
	DealsDatastore() Datastore

	// SetAPIAddr sets the address of the running API.
	SetAPIAddr(string) error

	// APIAddr returns the address of the running API.
	APIAddr() (string, error)

	// Version returns the current repo version.
	Version() uint

	// Path returns the repo path.
	Path() (string, error)

	// JournalPath returns the journal path.
	JournalPath() string

	// Close shuts down the repo.
	Close() error
}
