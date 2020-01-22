package wallet

import (
	"reflect"
	"strings"
	"sync"

	ds "github.com/ipfs/go-datastore"
	dsq "github.com/ipfs/go-datastore/query"
	"github.com/minio/blake2b-simd"
	"github.com/pkg/errors"

	bls "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

// DSBackendType is the reflect type of the DSBackend.
var DSBackendType = reflect.TypeOf(&DSBackend{})

// DSBackend is a wallet backend implementation for storing addresses in a datastore.
type DSBackend struct {
	lk sync.RWMutex

	// TODO: use a better interface that supports time locks, encryption, etc.
	ds repo.Datastore

	// TODO: proper cache
	cache map[address.Address]struct{}
}

var _ Backend = (*DSBackend)(nil)

// NewDSBackend constructs a new backend using the passed in datastore.
func NewDSBackend(ds repo.Datastore) (*DSBackend, error) {
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

	cache := make(map[address.Address]struct{})
	for _, el := range list {
		parsedAddr, err := address.NewFromString(strings.Trim(el.Key, "/"))
		if err != nil {
			return nil, errors.Wrapf(err, "trying to restore invalid address: %s", el.Key)
		}
		cache[parsedAddr] = struct{}{}
	}

	return &DSBackend{
		ds:    ds,
		cache: cache,
	}, nil
}

// ImportKey loads the address in `ai` and KeyInfo `ki` into the backend
func (backend *DSBackend) ImportKey(ki *types.KeyInfo) error {
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
	prv, err := crypto.GenerateKey()
	if err != nil {
		return address.Undef, err
	}

	// TODO: maybe the above call should just return a keyinfo?
	ki := &types.KeyInfo{
		PrivateKey:  prv,
		CryptSystem: types.SECP256K1,
	}

	if err := backend.putKeyInfo(ki); err != nil {
		return address.Undef, err
	}

	return ki.Address()
}

func (backend *DSBackend) newBLSAddress() (address.Address, error) {
	privateKey := bls.PrivateKeyGenerate()

	ki := &types.KeyInfo{
		PrivateKey:  privateKey[:],
		CryptSystem: types.BLS,
	}

	if err := backend.putKeyInfo(ki); err != nil {
		return address.Undef, err
	}

	return ki.Address()
}

func (backend *DSBackend) putKeyInfo(ki *types.KeyInfo) error {
	a, err := ki.Address()
	if err != nil {
		return err
	}

	backend.lk.Lock()
	defer backend.lk.Unlock()

	kib, err := ki.Marshal()
	if err != nil {
		return err
	}

	if err := backend.ds.Put(ds.NewKey(a.String()), kib); err != nil {
		return errors.Wrap(err, "failed to store new address")
	}

	backend.cache[a] = struct{}{}
	return nil
}

// SignBytes cryptographically signs `data` using the private key `priv`.
func (backend *DSBackend) SignBytes(data []byte, addr address.Address) (types.Signature, error) {
	ki, err := backend.GetKeyInfo(addr)
	if err != nil {
		return nil, err
	}

	if ki.CryptSystem == types.BLS {
		return crypto.SignBLS(ki.PrivateKey, data)
	}

	// sign the content
	hash := blake2b.Sum256(data)
	return crypto.SignSecp(ki.PrivateKey, hash[:])
}

// GetKeyInfo will return the private & public keys associated with address `addr`
// iff backend contains the addr.
func (backend *DSBackend) GetKeyInfo(addr address.Address) (*types.KeyInfo, error) {
	if !backend.HasAddress(addr) {
		return nil, errors.New("backend does not contain address")
	}

	// kib is a cbor of types.KeyInfo
	kib, err := backend.ds.Get(ds.NewKey(addr.String()))
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch private key from backend")
	}

	ki := &types.KeyInfo{}
	if err := ki.Unmarshal(kib); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal keyinfo from backend")
	}

	return ki, nil
}
