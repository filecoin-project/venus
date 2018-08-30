package repo

import (
	"os"
	"sync"

	keystore "gx/ipfs/QmTBWmvUbMDmvnZvzTpSjz6nVNJRiMMnj3JiFcgyJjvHaq/go-ipfs-keystore"
	"gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore"
	dss "gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore/sync"

	"github.com/filecoin-project/go-filecoin/config"
)

// MemRepo is an in memory implementation of the filecoin repo
type MemRepo struct {
	// lk guards the config
	lk         sync.RWMutex
	C          *config.Config
	D          Datastore
	Ks         keystore.Keystore
	W          Datastore
	version    uint
	apiAddress string
}

var _ Repo = (*MemRepo)(nil)

// NewInMemoryRepo makes a new one of these
func NewInMemoryRepo() *MemRepo {
	return &MemRepo{
		C:       config.NewDefaultConfig(),
		D:       dss.MutexWrap(datastore.NewMapDatastore()),
		Ks:      keystore.MutexWrap(keystore.NewMemKeystore()),
		W:       dss.MutexWrap(datastore.NewMapDatastore()),
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

// Version returns the version of the repo.
func (mr *MemRepo) Version() uint {
	return mr.version
}

// Close is a noop, just filling out the interface.
func (mr *MemRepo) Close() error {
	mr.CleanupSectorDirs()
	return nil
}

// StagingDir implements node.StagingDir.
func (mr *MemRepo) StagingDir() string {
	return "/tmp/sectors/staging/"
}

// SealedDir implements node.SectorDirs.
func (mr *MemRepo) SealedDir() string {
	return "/tmp/sectors/sealed/"
}

// CleanupSectorDirs removes all sector directories and their contents.
func (mr *MemRepo) CleanupSectorDirs() {
	os.RemoveAll(mr.StagingDir()) // nolint: errcheck
	os.RemoveAll(mr.SealedDir())  // nolint:errcheck
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
