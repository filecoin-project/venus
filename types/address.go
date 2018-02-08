package types

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"

	"github.com/pkg/errors"
)

// TODO: make address a little more sophisticated
type Address string

func (a Address) String() string {
	return "0x" + hex.EncodeToString([]byte(a))
}

func ParseAddress(s string) (Address, error) {
	if !strings.HasPrefix(s, "0x") {
		return "", fmt.Errorf("addresses must start with 0x")
	}
	raw, err := hex.DecodeString(s[2:])
	if err != nil {
		return "", errors.Wrap(err, "decoding address failed")
	}

	return Address(raw), nil
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
