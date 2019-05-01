package repo

import (
	"sync"

	"github.com/ipfs/go-datastore"
	dss "github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/go-ipfs-keystore"

	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/paths"
)

// MemRepo is an in-memory implementation of the Repo interface.
type MemRepo struct {
	// lk guards the config
	lk         sync.RWMutex
	C          *config.Config
	D          Datastore
	Ks         keystore.Keystore
	W          Datastore
	Chain      Datastore
	DealsDs    Datastore
	version    uint
	apiAddress string
}

var _ Repo = (*MemRepo)(nil)

// NewInMemoryRepo makes a new instance of MemRepo
func NewInMemoryRepo() *MemRepo {
	return &MemRepo{
		C:       config.NewDefaultConfig(),
		D:       dss.MutexWrap(datastore.NewMapDatastore()),
		Ks:      keystore.MutexWrap(keystore.NewMemKeystore()),
		W:       dss.MutexWrap(datastore.NewMapDatastore()),
		Chain:   dss.MutexWrap(datastore.NewMapDatastore()),
		DealsDs: dss.MutexWrap(datastore.NewMapDatastore()),
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

// Keystore returns the keystore.
func (mr *MemRepo) Keystore() keystore.Keystore {
	return mr.Ks
}

// WalletDatastore returns the wallet datastore.
func (mr *MemRepo) WalletDatastore() Datastore {
	return mr.W
}

// ChainDatastore returns the chain datastore.
func (mr *MemRepo) ChainDatastore() Datastore {
	return mr.Chain
}

// DealsDatastore returns the deals datastore for miners.
func (mr *MemRepo) DealsDatastore() Datastore {
	return mr.DealsDs
}

// Version returns the version of the repo.
func (mr *MemRepo) Version() uint {
	return mr.version
}

// Close deletes the temporary directories which hold staged piece data and
// sealed sectors.
func (mr *MemRepo) Close() error {
	return nil
}

// SetAPIAddr writes the address of the running API to memory.
func (mr *MemRepo) SetAPIAddr(addr string) error {
	mr.apiAddress = addr
	return nil
}

// APIAddr reads the address of the running API from memory.
func (mr *MemRepo) APIAddr() (string, error) {
	return mr.apiAddress, nil
}

// Path returns the default path.
func (mr *MemRepo) Path() (string, error) {
	return paths.GetRepoPath("")
}
