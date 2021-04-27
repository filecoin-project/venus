package repo

import (
	"errors"
	"sync"

	"github.com/filecoin-project/venus/pkg/repo/fskeystore"
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"

	"github.com/ipfs/go-datastore"
	dss "github.com/ipfs/go-datastore/sync"

	"github.com/filecoin-project/venus/app/paths"
	"github.com/filecoin-project/venus/pkg/config"
)

// MemRepo is an in-memory implementation of the repo interface.
type MemRepo struct {
	// lk guards the config
	lk    sync.RWMutex
	C     *config.Config
	D     blockstoreutil.Blockstore
	Ks    fskeystore.Keystore
	W     Datastore
	Chain Datastore
	Meta  Datastore
	Paych Datastore
	//Market     Datastore
	version    uint
	apiAddress string
	token      []byte
}

var _ Repo = (*MemRepo)(nil)

// NewInMemoryRepo makes a new instance of MemRepo
func NewInMemoryRepo() *MemRepo {
	defConfig := config.NewDefaultConfig()
	// Reduce the time it takes to encrypt wallet password, default ScryptN is 1 << 21
	// for test
	defConfig.Wallet.PassphraseConfig = config.TestPassphraseConfig()
	return &MemRepo{
		C:     defConfig,
		D:     blockstoreutil.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore())),
		Ks:    fskeystore.MutexWrap(fskeystore.NewMemKeystore()),
		W:     dss.MutexWrap(datastore.NewMapDatastore()),
		Chain: dss.MutexWrap(datastore.NewMapDatastore()),
		Meta:  dss.MutexWrap(datastore.NewMapDatastore()),
		Paych: dss.MutexWrap(datastore.NewMapDatastore()),
		//Market:  dss.MutexWrap(datastore.NewMapDatastore()),
		version: LatestVersion,
	}
}

// configModule returns the configuration object.
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
func (mr *MemRepo) Datastore() blockstoreutil.Blockstore {
	return mr.D
}

// Keystore returns the keystore.
func (mr *MemRepo) Keystore() fskeystore.Keystore {
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

// ChainDatastore returns the chain datastore.
func (mr *MemRepo) PaychDatastore() Datastore {
	return mr.Paych
}

/*// ChainDatastore returns the chain datastore.
func (mr *MemRepo) MarketDatastore() Datastore {
	return mr.Market
}
*/
// ChainDatastore returns the chain datastore.
func (mr *MemRepo) MetaDatastore() Datastore {
	return mr.Meta
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

func (mr *MemRepo) SetAPIToken(token []byte) error {
	if len(mr.token) == 0 {
		mr.token = token
	}
	return nil
}

func (mr *MemRepo) APIToken() (string, error) {
	if len(mr.token) == 0 {
		return "", errors.New("token not exists")
	}
	return string(mr.token), nil
}

// Path returns the default path.
func (mr *MemRepo) Path() (string, error) {
	return paths.GetRepoPath("")
}

// JournalPath returns a string to satisfy the repo interface.
func (mr *MemRepo) JournalPath() string {
	return "in_memory_filecoin_journal_path"
}

// repo return the repo
func (mr *MemRepo) Repo() Repo {
	return mr
}
