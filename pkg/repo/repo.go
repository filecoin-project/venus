package repo

import (
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/ipfs/go-datastore"
	keystore "github.com/ipfs/go-ipfs-keystore"

	"github.com/filecoin-project/venus/pkg/config"
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
	Datastore() blockstoreutil.Blockstore

	Keystore() keystore.Keystore

	// WalletDatastore is a specific storage solution, only used to store sensitive wallet information.
	WalletDatastore() Datastore

	// ChainDatastore is a specific storage solution, only used to store already validated chain data.
	ChainDatastore() Datastore

	// MetaDatastore is a specific storage solution, only used to store mpool data.
	MetaDatastore() Datastore

	// SetJsonrpcAPIAddr sets the address of the running jsonrpc API.
	SetJsonrpcAPIAddr(maddr string) error

	// SetRustfulAPIAddr sets the address of the running rustful API.
	SetRustfulAPIAddr(maddr string) error

	// APIAddr returns the address of the running API.
	APIAddr() (RpcAPI, error)

	// SetAPIToken set api token
	SetAPIToken(token []byte) error

	// Version returns the current repo version.
	Version() uint

	// Path returns the repo path.
	Path() (string, error)

	// JournalPath returns the journal path.
	JournalPath() string

	// Close shuts down the repo.
	Close() error

	// Repo return the repo
	Repo() Repo
}
