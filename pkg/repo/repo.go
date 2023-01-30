package repo

import (
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/repo/fskeystore"
	blockstoreutil "github.com/filecoin-project/venus/venus-shared/blockstore"
	"github.com/ipfs/go-datastore"
)

// Datastore is the datastore interface provided by the repo
type Datastore datastore.Batching

// repo is a representation of all persistent data in a filecoin node.
type Repo interface {
	Config() *config.Config
	// ReplaceConfig replaces the current config, with the newly passed in one.
	ReplaceConfig(cfg *config.Config) error

	// Datastore is a general storage solution for things like blocks.
	Datastore() blockstoreutil.Blockstore

	Keystore() fskeystore.Keystore

	// WalletDatastore is a specific storage solution, only used to store sensitive wallet information.
	WalletDatastore() Datastore

	// ChainDatastore is a specific storage solution, only used to store already validated chain data.
	ChainDatastore() Datastore

	// MetaDatastore is a specific storage solution, only used to store mpool data.
	MetaDatastore() Datastore

	// MarketDatastore() Datastore

	PaychDatastore() Datastore
	// SetJsonrpcAPIAddr sets the address of the running jsonrpc API.
	SetAPIAddr(maddr string) error

	// APIAddr returns the address of the running API.
	APIAddr() (string, error)

	// SetAPIToken set api token
	SetAPIToken(token []byte) error

	// APIToken get api token
	APIToken() (string, error)

	// Version returns the current repo version.
	Version() uint

	// Path returns the repo path.
	Path() (string, error)

	// JournalPath returns the journal path.
	JournalPath() string

	// SqlitePath returns the path for the Sqlite database
	SqlitePath() (string, error)

	// Close shuts down the repo.
	Close() error

	// repo return the repo
	Repo() Repo
}
