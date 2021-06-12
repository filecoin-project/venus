package wallet

import (
	"crypto/rand"
	"reflect"
	"strings"
	"sync"

	"github.com/awnumar/memguard"
	"github.com/filecoin-project/go-address"
	ds "github.com/ipfs/go-datastore"
	dsq "github.com/ipfs/go-datastore/query"
	"github.com/pborman/uuid"
	"github.com/pkg/errors"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/repo"
)

const (
	undetermined = iota

	Lock
	Unlock
)

var ErrInvalidPassword = errors.New("password matching failed")
var ErrRepeatPassword = errors.New("set password more than once")

// DSBackendType is the reflect type of the DSBackend.
var DSBackendType = reflect.TypeOf(&DSBackend{})

// DSBackend is a wallet backend implementation for storing addresses in a datastore.
type DSBackend struct {
	lk sync.RWMutex

	// TODO: use a better interface that supports time locks, encryption, etc.
	ds repo.Datastore

	cache map[address.Address]struct{}

	PassphraseConf config.PassphraseConfig

	password *memguard.Enclave
	unLocked map[address.Address]*crypto.KeyInfo

	state int
}

var _ Backend = (*DSBackend)(nil)

// NewDSBackend constructs a new backend using the passed in datastore.
func NewDSBackend(ds repo.Datastore, passphraseCfg config.PassphraseConfig, password []byte) (*DSBackend, error) {
	result, err := ds.Query(dsq.Query{
		KeysOnly: true,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to query datastore")
	}

	list, err := result.Rest()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read query results")
	}

	addrCache := make(map[address.Address]struct{}, len(list))
	for _, el := range list {
		parsedAddr, err := address.NewFromString(strings.Trim(el.Key, "/"))
		if err != nil {
			return nil, errors.Wrapf(err, "trying to restore invalid address: %s", el.Key)
		}
		addrCache[parsedAddr] = struct{}{}
	}

	backend := &DSBackend{
		ds:             ds,
		cache:          addrCache,
		PassphraseConf: passphraseCfg,
		unLocked:       make(map[address.Address]*crypto.KeyInfo, len(addrCache)),
	}

	if len(password) != 0 {
		if err := backend.SetPassword(password); err != nil {
			return nil, err
		}
	}

	return backend, nil
}

// ImportKey loads the address in `ai` and KeyInfo `ki` into the backend
func (backend *DSBackend) ImportKey(ki *crypto.KeyInfo) error {
	return backend.putKeyInfo(ki)
}

// Addresses returns a list of all addresses that are stored in this backend.
func (backend *DSBackend) Addresses() []address.Address {
	backend.lk.RLock()
	defer backend.lk.RUnlock()

	var cpy []address.Address
	for addr := range backend.cache {
		cpy = append(cpy, addr)
	}
	return cpy
}

// HasAddress checks if the passed in address is stored in this backend.
// Safe for concurrent access.
func (backend *DSBackend) HasAddress(addr address.Address) bool {
	backend.lk.RLock()
	defer backend.lk.RUnlock()

	_, ok := backend.cache[addr]
	return ok
}

// NewAddress creates a new address and stores it.
// Safe for concurrent access.
func (backend *DSBackend) NewAddress(protocol address.Protocol) (address.Address, error) {
	switch protocol {
	case address.BLS:
		return backend.newBLSAddress()
	case address.SECP256K1:
		return backend.newSecpAddress()
	default:
		return address.Undef, errors.Errorf("Unknown address protocol %d", protocol)
	}
}

func (backend *DSBackend) newSecpAddress() (address.Address, error) {
	ki, err := crypto.NewSecpKeyFromSeed(rand.Reader)
	if err != nil {
		return address.Undef, err
	}

	if err := backend.putKeyInfo(&ki); err != nil {
		return address.Undef, err
	}
	return ki.Address()
}

func (backend *DSBackend) newBLSAddress() (address.Address, error) {
	ki, err := crypto.NewBLSKeyFromSeed(rand.Reader)
	if err != nil {
		return address.Undef, err
	}

	if err := backend.putKeyInfo(&ki); err != nil {
		return address.Undef, err
	}
	return ki.Address()
}

func (backend *DSBackend) putKeyInfo(ki *crypto.KeyInfo) error {
	addr, err := ki.Address()
	if err != nil {
		return err
	}

	key := &Key{
		ID:      uuid.NewRandom(),
		Address: addr,
		KeyInfo: ki,
	}

	var keyJSON []byte
	err = backend.UsePassword(func(password []byte) error {
		var err error
		keyJSON, err = encryptKey(key, password, backend.PassphraseConf.ScryptN, backend.PassphraseConf.ScryptP)
		return err
	})
	if err != nil {
		return err
	}

	backend.lk.Lock()
	if err := backend.ds.Put(ds.NewKey(key.Address.String()), keyJSON); err != nil {
		return errors.Wrapf(err, "failed to store new address: %s", key.Address.String())
	}
	backend.cache[addr] = struct{}{}
	backend.unLocked[addr] = ki
	backend.lk.Unlock()

	return nil
}

// SignBytes cryptographically signs `data` using the private key `priv`.
func (backend *DSBackend) SignBytes(data []byte, addr address.Address) (*crypto.Signature, error) {
	backend.lk.Lock()
	ki, found := backend.unLocked[addr]
	backend.lk.Unlock()
	if !found {
		return nil, errors.Errorf("%s is locked", addr.String())
	}

	var signature *crypto.Signature
	err := ki.UsePrivateKey(func(privateKey []byte) error {
		var err error
		signature, err = crypto.Sign(data, privateKey, ki.SigType)
		return err
	})
	return signature, err
}

// GetKeyInfo will return the private & public keys associated with address `addr`
// iff backend contains the addr.
func (backend *DSBackend) GetKeyInfo(addr address.Address) (*crypto.KeyInfo, error) {
	if !backend.HasAddress(addr) {
		return nil, errors.New("backend does not contain address")
	}

	var key *Key
	err := backend.UsePassword(func(password []byte) error {
		var err error
		key, err = backend.getKey(addr, password)

		return err
	})
	if err != nil {
		return nil, err
	}

	return key.KeyInfo, nil
}

func (backend *DSBackend) GetKeyInfoPassphrase(addr address.Address, password []byte) (*crypto.KeyInfo, error) {
	defer func() {
		for i := range password {
			password[i] = 0
		}
	}()
	if !backend.HasAddress(addr) {
		return nil, errors.New("backend does not contain address")
	}

	key, err := backend.getKey(addr, password)
	if err != nil {
		return nil, err
	}

	return key.KeyInfo, nil
}

func (backend *DSBackend) getKey(addr address.Address, password []byte) (*Key, error) {
	b, err := backend.ds.Get(ds.NewKey(addr.String()))
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch private key from backend")
	}

	return decryptKey(b, password)
}

func (backend *DSBackend) LockWallet() error {
	if backend.state == Lock {
		return xerrors.Errorf("already locked")
	}

	if len(backend.Addresses()) == 0 {
		return xerrors.Errorf("no address need lock")
	}

	for _, addr := range backend.Addresses() {
		backend.lk.Lock()
		delete(backend.unLocked, addr)
		backend.lk.Unlock()
	}
	backend.cleanPassword()
	backend.state = Lock

	return nil
}

func (backend *DSBackend) UnLockWallet(password []byte) error {
	defer func() {
		for i := range password {
			password[i] = 0
		}
	}()
	if backend.state == Unlock {
		return xerrors.Errorf("already unlocked")
	}

	if len(backend.Addresses()) == 0 {
		return xerrors.Errorf("no address need unlock")
	}

	for _, addr := range backend.Addresses() {
		ki, err := backend.GetKeyInfoPassphrase(addr, password)
		if err != nil {
			return err
		}

		backend.lk.Lock()
		backend.unLocked[addr] = ki
		backend.lk.Unlock()
	}
	backend.state = Unlock

	return nil
}

func (backend *DSBackend) SetPassword(password []byte) error {
	if backend.password != nil {
		return ErrRepeatPassword
	}

	for _, addr := range backend.Addresses() {
		ki, err := backend.GetKeyInfoPassphrase(addr, password)
		if err != nil {
			return err
		}
		backend.lk.Lock()
		backend.unLocked[addr] = ki
		backend.lk.Unlock()
	}
	if backend.state == undetermined {
		backend.state = Unlock
	}

	backend.setPassword(password)

	return nil
}

func (backend *DSBackend) HasPassword() bool {
	return backend.password != nil
}

func (backend *DSBackend) WalletState() int {
	return backend.state
}

func (backend *DSBackend) setPassword(password []byte) {
	backend.lk.Lock()
	defer backend.lk.Unlock()

	backend.password = memguard.NewEnclave(password)
}

func (backend *DSBackend) UsePassword(f func(password []byte) error) error {
	if backend.password == nil {
		return f([]byte{})
	}
	buf, err := backend.password.Open()
	if err != nil {
		return err
	}
	defer buf.Destroy()

	return f(buf.Bytes())
}

func (backend *DSBackend) cleanPassword() {
	backend.lk.Lock()
	defer backend.lk.Unlock()
	backend.password = nil
}
