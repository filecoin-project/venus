package wallet

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"sync"

	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

var (
	// ErrUnknownAddress is returned when the given address is not stored in this wallet.
	ErrUnknownAddress = errors.New("unknown address")
)

// Wallet manages the locally stored addresses.
type Wallet struct {
	lk sync.Mutex

	backends map[reflect.Type][]Backend
}

// New constructs a new wallet, that manages addresses in all the
// passed in backends.
func New(backends ...Backend) *Wallet {
	backendsMap := make(map[reflect.Type][]Backend)

	for _, backend := range backends {
		kind := reflect.TypeOf(backend)
		backendsMap[kind] = append(backendsMap[kind], backend)
	}

	return &Wallet{
		backends: backendsMap,
	}
}

// HasAddress checks if the given address is stored.
// Safe for concurrent access.
func (w *Wallet) HasAddress(a address.Address) bool {
	_, err := w.Find(a)
	return err == nil
}

// Find searches through all backends and returns the one storing the passed
// in address.
// Safe for concurrent access.
func (w *Wallet) Find(addr address.Address) (Backend, error) {
	w.lk.Lock()
	defer w.lk.Unlock()

	for _, backends := range w.backends {
		for _, backend := range backends {
			if backend.HasAddress(addr) {
				return backend, nil
			}
		}
	}

	return nil, ErrUnknownAddress
}

// Addresses retrieves all stored addresses.
// Safe for concurrent access.
// Always sorted in the same order.
func (w *Wallet) Addresses() []address.Address {
	w.lk.Lock()
	defer w.lk.Unlock()

	var out []address.Address
	for _, backends := range w.backends {
		for _, backend := range backends {
			out = append(out, backend.Addresses()...)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return bytes.Compare(out[i].Bytes(), out[j].Bytes()) < 0
	})

	return out
}

// Backends returns backends by their kind.
func (w *Wallet) Backends(kind reflect.Type) []Backend {
	w.lk.Lock()
	defer w.lk.Unlock()

	cpy := make([]Backend, len(w.backends[kind]))
	copy(cpy, w.backends[kind])
	return cpy
}

// SignBytes cryptographically signs `data` using the private key corresponding to
// address `addr`
func (w *Wallet) SignBytes(data []byte, addr address.Address) (types.Signature, error) {
	// Check that we are storing the address to sign for.
	backend, err := w.Find(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "could not find address: %s", addr)
	}
	return backend.SignBytes(data, addr)
}

// NewAddress creates a new account address on the default wallet backend.
func NewAddress(w *Wallet, p address.Protocol) (address.Address, error) {
	backends := w.Backends(DSBackendType)
	if len(backends) == 0 {
		return address.Undef, fmt.Errorf("missing default ds backend")
	}

	backend := (backends[0]).(*DSBackend)
	return backend.NewAddress(p)
}

// GetPubKeyForAddress returns the public key in the keystore associated with
// the given address.
func (w *Wallet) GetPubKeyForAddress(addr address.Address) ([]byte, error) {
	info, err := w.keyInfoForAddr(addr)
	if err != nil {
		return nil, err
	}

	return info.PublicKey(), nil
}

// NewKeyInfo creates a new KeyInfo struct in the wallet backend and returns it
func (w *Wallet) NewKeyInfo() (*types.KeyInfo, error) {
	newAddr, err := NewAddress(w, address.SECP256K1)
	if err != nil {
		return &types.KeyInfo{}, err
	}

	return w.keyInfoForAddr(newAddr)
}

func (w *Wallet) keyInfoForAddr(addr address.Address) (*types.KeyInfo, error) {
	backend, err := w.Find(addr)
	if err != nil {
		return &types.KeyInfo{}, err
	}

	info, err := backend.GetKeyInfo(addr)
	if err != nil {
		return &types.KeyInfo{}, err
	}
	return info, nil
}

// Import adds the given keyinfos to the wallet
func (w *Wallet) Import(kinfos ...*types.KeyInfo) ([]address.Address, error) {
	dsb := w.Backends(DSBackendType)
	if len(dsb) != 1 {
		return nil, fmt.Errorf("expected exactly one datastore wallet backend")
	}

	imp, ok := dsb[0].(Importer)
	if !ok {
		return nil, fmt.Errorf("datastore backend wallets should implement importer")
	}

	var out []address.Address
	for _, ki := range kinfos {
		if err := imp.ImportKey(ki); err != nil {
			return nil, err
		}

		a, err := ki.Address()
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

// Export returns the KeyInfos for the given wallet addresses
func (w *Wallet) Export(addrs []address.Address) ([]*types.KeyInfo, error) {
	out := make([]*types.KeyInfo, len(addrs))
	for i, addr := range addrs {
		bck, err := w.Find(addr)
		if err != nil {
			return nil, err
		}

		ki, err := bck.GetKeyInfo(addr)
		if err != nil {
			return nil, err
		}
		out[i] = ki
	}

	return out, nil
}
