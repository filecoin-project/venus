package wallet

import (
	"crypto/rand"
	"fmt"
	"sync"

	"github.com/filecoin-project/go-filecoin/types"
)

// Wallet manages the locally stored accounts.
type Wallet struct {
	lk             sync.Mutex
	addresses      map[string]types.Address
	defaultAddress types.Address
}

// New creates a new Wallet.
func New() *Wallet {
	return &Wallet{
		addresses: make(map[string]types.Address),
	}
}

// HasAddress checks if the given address is stored.
// Safe for concurrent access.
func (w *Wallet) HasAddress(a types.Address) bool {
	w.lk.Lock()
	defer w.lk.Unlock()
	_, ok := w.addresses[a.String()]
	return ok
}

// NewAddress creates a new address and stores it.
// Safe for concurrent access.
func (w *Wallet) NewAddress() types.Address {
	fakeAddrBuf := make([]byte, 20)
	_, err := rand.Read(fakeAddrBuf)
	if err != nil {
		// if rand.Read errors we have a problem
		panic(err)
	}

	hash, err := types.AddressHash(fakeAddrBuf)
	if err != nil {
		panic(err)
	}

	fakeAddr := types.NewMainnetAddress(hash)

	w.lk.Lock()
	defer w.lk.Unlock()

	w.addresses[fakeAddr.String()] = fakeAddr

	if w.defaultAddress == (types.Address{}) {
		w.defaultAddress = fakeAddr
	}

	return fakeAddr
}

// GetAddresses retrieves all stored addresses.
// Safe for concurrent access.
func (w *Wallet) GetAddresses() []types.Address {
	w.lk.Lock()
	defer w.lk.Unlock()
	out := make([]types.Address, 0, len(w.addresses))
	for _, a := range w.addresses {
		out = append(out, a)
	}
	return out
}

// GetDefaultAddress retrieves the default address if one exists.
// If no address is available it returns an error.
// Safe for concurrent access.
func (w *Wallet) GetDefaultAddress() (types.Address, error) {
	w.lk.Lock()
	defer w.lk.Unlock()

	if w.defaultAddress == (types.Address{}) {
		return types.Address{}, fmt.Errorf("no default address in local wallet")
	}

	return w.defaultAddress, nil
}
