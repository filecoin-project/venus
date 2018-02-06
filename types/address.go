package types

import (
	"crypto/rand"
	"sync"
)

// TODO: make address a little more sophisticated
type Address string

type Wallet struct {
	lk        sync.Mutex
	Addresses map[Address]struct{}
}

func NewWallet() *Wallet {
	return &Wallet{
		Addresses: make(map[Address]struct{}),
	}
}

func (w *Wallet) HasAddress(a Address) bool {
	_, ok := w.Addresses[a]
	return ok
}

func (w *Wallet) NewAddress() Address {
	fakeAddrBuf := make([]byte, 20)
	rand.Read(fakeAddrBuf)
	fakeAddr := Address(fakeAddrBuf)

	w.lk.Lock()
	defer w.lk.Unlock()
	w.Addresses[fakeAddr] = struct{}{}
	return fakeAddr
}
