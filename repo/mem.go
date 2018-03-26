package repo

import (
	"gx/ipfs/QmPpegoMqhAEqjncrzArm7KVWAkCm78rqL2DPuNjhPrshg/go-datastore"

	"github.com/filecoin-project/go-filecoin/config"
)

// MemRepo is an in memory implementation of the filecoin repo
type MemRepo struct {
	C       *config.Config
	D       Datastore
	version uint
}

var _ Repo = (*MemRepo)(nil)

// NewInMemoryRepo makes a new one of these
func NewInMemoryRepo() *MemRepo {
	return &MemRepo{
		C:       config.NewDefaultConfig(),
		D:       datastore.NewMapDatastore(),
		version: Version,
	}
}

// Config returns the configuration object
func (mr *MemRepo) Config() *config.Config {
	return mr.C
}

// Datastore returns the datastore
func (mr *MemRepo) Datastore() Datastore {
	return mr.D
}

// Version returns the version of the repo
func (mr *MemRepo) Version() uint {
	return mr.version
}

// Close is a noop, just filling out the interface
func (mr *MemRepo) Close() error {
	return nil
}
