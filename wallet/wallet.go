package wallet

import (
	"crypto/rand"
	"sync"

	"github.com/filecoin-project/go-filecoin/types"
)

type Wallet struct {
	lk        sync.Mutex
	addresses map[types.Address]struct{}
}

func New() *Wallet {
	return &Wallet{
		addresses: make(map[types.Address]struct{}),
	}
}

func (w *Wallet) HasAddress(a types.Address) bool {
	w.lk.Lock()
	defer w.lk.Unlock()
	_, ok := w.addresses[a]
	return ok
}

func (w *Wallet) NewAddress() types.Address {
	fakeAddrBuf := make([]byte, 20)
	rand.Read(fakeAddrBuf)
	fakeAddr := types.Address(fakeAddrBuf)

	w.lk.Lock()
	defer w.lk.Unlock()
	w.addresses[fakeAddr] = struct{}{}
	return fakeAddr
}

func (w *Wallet) GetAddresses() []types.Address {
	w.lk.Lock()
	defer w.lk.Unlock()
	out := make([]types.Address, 0, len(w.addresses))
	for a := range w.addresses {
		out = append(out, a)
	}
	return out
}
