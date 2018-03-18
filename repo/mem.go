package repo

import (
	"github.com/filecoin-project/go-filecoin/config"
	"gx/ipfs/QmPpegoMqhAEqjncrzArm7KVWAkCm78rqL2DPuNjhPrshg/go-datastore"
)

// MemRepo is an in memory implementation of the filecoin repo
type MemRepo struct {
	C *config.Config
	D Datastore
}

// NewInMemoryRepo makes a new one of these
func NewInMemoryRepo() *MemRepo {
	return &MemRepo{
		C: &config.Config{},
		D: datastore.NewMapDatastore(),
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
