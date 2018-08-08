package wallet

import (
	"crypto/ecdsa"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	ds "gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore"
	dsq "gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore/query"

	"github.com/filecoin-project/go-filecoin/crypto"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
	wutil "github.com/filecoin-project/go-filecoin/wallet/util"
)

const (
	// SECP256K1 is a curve used to computer private keys
	SECP256K1 = "secp256k1"
)

// DSBackendType is the reflect type of the DSBackend.
var DSBackendType = reflect.TypeOf(&DSBackend{})

// DSBackend is a wallet backend implementation for storing addresses in a datastore.
type DSBackend struct {
	lk sync.RWMutex

	// TODO: use a better interface that supports time locks, encryption, etc.
	ds repo.Datastore

	// TODO: proper cache
	cache map[types.Address]struct{}
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

	cache := make(map[types.Address]struct{})
	for _, el := range list {
		parsedAddr, err := types.NewAddressFromString(strings.Trim(el.Key, "/"))
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
func (backend *DSBackend) Addresses() []types.Address {
	backend.lk.RLock()
	defer backend.lk.RUnlock()

	var cpy []types.Address
	for addr := range backend.cache {
		cpy = append(cpy, addr)
	}
	return cpy
}

// HasAddress checks if the passed in address is stored in this backend.
// Safe for concurrent access.
func (backend *DSBackend) HasAddress(addr types.Address) bool {
	backend.lk.RLock()
	defer backend.lk.RUnlock()

	_, ok := backend.cache[addr]
	return ok
}

// NewAddress creates a new address and stores it.
// Safe for concurrent access.
func (backend *DSBackend) NewAddress() (types.Address, error) {
	prv, err := crypto.GenerateKey()
	if err != nil {
		return types.Address{}, err
	}

	// TODO: maybe the above call should just return a keyinfo?
	ki := &types.KeyInfo{
		PrivateKey: crypto.ECDSAToBytes(prv),
		Curve:      SECP256K1,
	}

	if err := backend.putKeyInfo(ki); err != nil {
		return types.Address{}, err
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
func (backend *DSBackend) SignBytes(data []byte, addr types.Address) (types.Signature, error) {
	ki, err := backend.GetKeyInfo(addr)
	if err != nil {
		return nil, err
	}

	privateKey, _, err := keysFromInfo(ki)
	if err != nil {
		return nil, err
	}

	return wutil.Sign(privateKey, data)
}

// Verify cryptographically verifies that 'sig' is the signed hash of 'data' with
// the public key `pk`.
func (backend *DSBackend) Verify(data []byte, pk []byte, sig types.Signature) (bool, error) {
	return wutil.Verify(pk, data, sig)
}

// Ecrecover returns an uncompressed public key that could produce the given
// signature from data.
// Note: The returned public key should not be used to verify `data` is valid
// since a public key may have N private key pairs
func (backend *DSBackend) Ecrecover(data []byte, sig types.Signature) ([]byte, error) {
	return wutil.Ecrecover(data, sig)
}

// GetKeyInfo will return the private & public keys associated with address `addr`
// iff backend contains the addr.
func (backend *DSBackend) GetKeyInfo(addr types.Address) (*types.KeyInfo, error) {
	if !backend.HasAddress(addr) {
		return nil, errors.New("backend does not contain address")
	}

	// kib is a cbor of types.KeyInfo
	kib, err := backend.ds.Get(ds.NewKey(addr.String()))
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch private key from backend")
	}

	ki := &types.KeyInfo{}
	if err := ki.Unmarshal(kib.([]byte)); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal keyinfo from backend")
	}

	return ki, nil
}

func keysFromInfo(ki *types.KeyInfo) (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	// Developer error if we add a new type and don't update this method
	if ki.Type() != SECP256K1 {
		panic(fmt.Sprintf("unknown key type %s", ki.Type()))
	}

	prv, err := crypto.BytesToECDSA(ki.Key())
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to unmarshal private key")
	}

	return prv, &prv.PublicKey, nil
}
