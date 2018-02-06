package types

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
)

// TODO: make address a little more sophisticated
type Address string

func (a Address) String() string {
	return "0x" + hex.EncodeToString([]byte(a))
}

type Wallet struct {
	lk        sync.Mutex
	addresses map[Address]struct{}
}

func NewWallet() *Wallet {
	return &Wallet{
		addresses: make(map[Address]struct{}),
	}
}

func (w *Wallet) HasAddress(a Address) bool {
	w.lk.Lock()
	defer w.lk.Unlock()
	_, ok := w.addresses[a]
	return ok
}

func (w *Wallet) NewAddress() Address {
	fakeAddrBuf := make([]byte, 20)
	rand.Read(fakeAddrBuf)
	fakeAddr := Address(fakeAddrBuf)

	w.lk.Lock()
	defer w.lk.Unlock()
	w.addresses[fakeAddr] = struct{}{}
	return fakeAddr
}

func (w *Wallet) GetAddresses() []Address {
	w.lk.Lock()
	defer w.lk.Unlock()
	out := make([]Address, 0, len(w.addresses))
	for a := range w.addresses {
		out = append(out, a)
	}
	return out
}
