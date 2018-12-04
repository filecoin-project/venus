package repo

import (
	"io/ioutil"
	"os"
	"sync"

	"gx/ipfs/QmZxaF6uz9VWbuQ5Jk43stXksbnX8x5veYS73eFD4hKqtD/go-ipfs-keystore"
	"gx/ipfs/Qmf4xQhNomPNhrtZc67qSnfJSjxjXs9LWvknJtSXwimPrM/go-datastore"
	dss "gx/ipfs/Qmf4xQhNomPNhrtZc67qSnfJSjxjXs9LWvknJtSXwimPrM/go-datastore/sync"

	"github.com/filecoin-project/go-filecoin/config"
)

// MemRepo is a mostly (see `stagingDir` and `sealedDir`) in-memory
// implementation of the Repo interface.
type MemRepo struct {
	// lk guards the config
	lk         sync.RWMutex
	C          *config.Config
	D          Datastore
	Ks         keystore.Keystore
	W          Datastore
	Chain      Datastore
	version    uint
	apiAddress string
	stagingDir string
	sealedDir  string
}

var _ Repo = (*MemRepo)(nil)

// NewInMemoryRepo makes a new instance of MemRepo
func NewInMemoryRepo() *MemRepo {
	stagingDir, err := ioutil.TempDir("", "staging")
	if err != nil {
		panic(err)
	}

	sealedDir, err := ioutil.TempDir("", "staging")
	if err != nil {
		panic(err)
	}

	return &MemRepo{
		C:          config.NewDefaultConfig(),
		D:          dss.MutexWrap(datastore.NewMapDatastore()),
		Ks:         keystore.MutexWrap(keystore.NewMemKeystore()),
		W:          dss.MutexWrap(datastore.NewMapDatastore()),
		Chain:      dss.MutexWrap(datastore.NewMapDatastore()),
		version:    Version,
		stagingDir: stagingDir,
		sealedDir:  sealedDir,
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

// Version returns the version of the repo.
func (mr *MemRepo) Version() uint {
	return mr.version
}

// Close deletes the temporary directories which hold staged piece data and
// sealed sectors.
func (mr *MemRepo) Close() error {
	mr.CleanupSectorDirs()
	return nil
}

// StagingDir implements node.StagingDir.
func (mr *MemRepo) StagingDir() string {
	return mr.stagingDir
}

// SealedDir implements node.SectorDirs.
func (mr *MemRepo) SealedDir() string {
	return mr.sealedDir
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
