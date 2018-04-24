package repo

import (
	"sync"

	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/keystore"
	kss "github.com/filecoin-project/go-filecoin/keystore/sync"

	"gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"
	dss "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore/sync"
)

// MemRepo is an in memory implementation of the filecoin repo
type MemRepo struct {
	// lk guards the config
	lk      sync.RWMutex
	C       *config.Config
	D       Datastore
	Ks      keystore.Keystore
	version uint
}

var _ Repo = (*MemRepo)(nil)

// NewInMemoryRepo makes a new one of these
func NewInMemoryRepo() *MemRepo {
	return &MemRepo{
		C:       config.NewDefaultConfig(),
		D:       dss.MutexWrap(datastore.NewMapDatastore()),
		Ks:      kss.MutexWrap(keystore.NewMemKeystore()),
		version: Version,
	}
}

// Config returns the configuration object.
func (mr *MemRepo) Config() *config.Config {
	mr.lk.RLock()
	defer mr.lk.RUnlock()

	return mr.C
}

// ReplaceConfig replaces the current config with the newly passed in one.
func (mr *MemRepo) ReplaceConfig(cfg *config.Config) error {
	mr.lk.Lock()
	defer mr.lk.Unlock()

	mr.C = cfg

	return nil
}

// Datastore returns the datastore.
func (mr *MemRepo) Datastore() Datastore {
	return mr.D
}

// Keystore returns the keystore
func (mr *MemRepo) Keystore() keystore.Keystore {
	return mr.Ks
}

// Version returns the version of the repo
func (mr *MemRepo) Version() uint {
	return mr.version
}

// Close is a noop, just filling out the interface
func (mr *MemRepo) Close() error {
	return nil
}
