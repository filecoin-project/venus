package wallet

import (
	"crypto/rand"
	"reflect"
	"strings"
	"sync"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	ds "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"
	dsq "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore/query"
	ci "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"

	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
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
	// Generate a key pair
	priv, pub, err := ci.GenerateSecp256k1Key(rand.Reader)
	if err != nil {
		return types.Address{}, err
	}

	bpub, err := pub.Bytes()
	if err != nil {
		return types.Address{}, err
	}

	// address is based on the public key this is what allows you to get
	// money out of the actor, if you have the matching priv for the address
	adderHash, err := types.AddressHash(bpub)
	if err != nil {
		return types.Address{}, err
	}
	// TODO: use the address type we are running on from the config
	newAdder := types.NewMainnetAddress(adderHash)

	bpriv, err := priv.Bytes()
	if err != nil {
		return types.Address{}, err
	}

	backend.lk.Lock()
	defer backend.lk.Unlock()

	// persist address (pubkey) and its corresponding priv key
	if err := backend.ds.Put(ds.NewKey(newAdder.String()), bpriv); err != nil {
		return types.Address{}, errors.Wrap(err, "failed to store new address")
	}

	backend.cache[newAdder] = struct{}{}

	return newAdder, nil
}
