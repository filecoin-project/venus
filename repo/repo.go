package repo

import (
	"gx/ipfs/QmbRDw6b2xXnZywek9E8qk1z4NAVrGxqZvbvwfSioiSy1S/go-ipfs-keystore"
	"gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore"

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
	// ReplaceConfig replaces the current config, with the newly passed in one.
	ReplaceConfig(cfg *config.Config) error

	// Datastore is a general storage solution for things like blocks.
	Datastore() Datastore
	Keystore() keystore.Keystore

	// WalletDatastore is a specifc storage solution, only used to store sensitive wallet information.
	WalletDatastore() Datastore

	// SetAPIAddr sets the address of the running API.
	SetAPIAddr(string) error

	// APIAddr returns the address of the running API.
	APIAddr() (string, error)

	Version() uint
	Close() error
}
