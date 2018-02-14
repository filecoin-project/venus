package wallet

import (
	"crypto/rand"
	"sync"

	"github.com/filecoin-project/go-filecoin/types"
)

// Wallet manages the locally stored accounts.
type Wallet struct {
	lk        sync.Mutex
	addresses map[types.Address]struct{}
}

// New creates a new Wallet.
func New() *Wallet {
	return &Wallet{
		addresses: make(map[types.Address]struct{}),
	}
}

// HasAddress checks if the given address is stored.
// Safe for concurrent access.
func (w *Wallet) HasAddress(a types.Address) bool {
	w.lk.Lock()
	defer w.lk.Unlock()
	_, ok := w.addresses[a]
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

	fakeAddr := types.Address(fakeAddrBuf)

	w.lk.Lock()
	defer w.lk.Unlock()
	w.addresses[fakeAddr] = struct{}{}
	return fakeAddr
}

// GetAddresses retrieves all stored addresses.
// Safe for concurrent access.
func (w *Wallet) GetAddresses() []types.Address {
	w.lk.Lock()
	defer w.lk.Unlock()
	out := make([]types.Address, 0, len(w.addresses))
	for a := range w.addresses {
		out = append(out, a)
	}
	return out
}
