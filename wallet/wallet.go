package wallet

import (
	"fmt"
	"reflect"
	"sync"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
	wutil "github.com/filecoin-project/go-filecoin/wallet/util"
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
// Note that the Golang runtime randomizes map iteration order, so the order in
// which addresses appear in the returned list may differ across Addresses()
// calls for the same wallet.
// TODO: Should we make this ordering deterministic?
func (w *Wallet) Addresses() []address.Address {
	w.lk.Lock()
	defer w.lk.Unlock()

	var out []address.Address
	for _, backends := range w.backends {
		for _, backend := range backends {
			out = append(out, backend.Addresses()...)
		}
	}

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
		return nil, errors.Wrapf(err, "failed to sign data with address: %s", addr)
	}
	return backend.SignBytes(data, addr)
}

// Verify cryptographically verifies that 'sig' is the signed hash of 'data' with
// the public key `pk`.
func (w *Wallet) Verify(data []byte, pk []byte, sig types.Signature) (bool, error) {
	return wutil.Verify(pk, data, sig)
}

// Ecrecover returns an uncompressed public key that could produce the given
// signature from data.
// Note: The returned public key should not be used to verify `data` is valid
// since a public key may have N private key pairs
func (w *Wallet) Ecrecover(data []byte, sig types.Signature) ([]byte, error) {
	return wutil.Ecrecover(data, sig)
}

// NewAddress creates a new account address on the default wallet backend.
func NewAddress(w *Wallet) (address.Address, error) {
	backends := w.Backends(DSBackendType)
	if len(backends) == 0 {
		return address.Address{}, fmt.Errorf("missing default ds backend")
	}

	backend := (backends[0]).(*DSBackend)
	return backend.NewAddress()
}

// GetPubKeyForAddress returns the public key in the keystore associated with
// the given address.
func (w *Wallet) GetPubKeyForAddress(addr address.Address) ([]byte, error) {
	info, err := w.keyInfoForAddr(addr)
	if err != nil {
		return nil, err
	}
	pubkey, err := info.PublicKey()
	if err != nil {
		return nil, err
	}
	return pubkey, nil
}

// NewKeyInfo creates a new KeyInfo struct in the wallet backend and returns it
func (w *Wallet) NewKeyInfo() (*types.KeyInfo, error) {
	newAddr, err := NewAddress(w)
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
