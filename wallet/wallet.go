package wallet

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"sync"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmZp3eKdYQHHAneECmeK6HhiMwTPufmjC8DuuaGKv3unvx/blake2b-simd"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/proofs"
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
		return nil, errors.Wrapf(err, "failed to sign data with address: %s", addr)
	}
	return backend.SignBytes(data, addr)
}

// GetAddressForPubKey looks up a KeyInfo address associated with a given PublicKey
func (w *Wallet) GetAddressForPubKey(pk []byte) (address.Address, error) {
	var addr address.Address
	addrs := w.Addresses()
	for _, addr = range addrs {
		testPk, err := w.GetPubKeyForAddress(addr)
		if err != nil {
			return addr, errors.New("could not fetch public key")
		}

		if bytes.Equal(testPk, pk) {
			return addr, nil
		}
	}
	return addr, errors.New("public key not found in wallet")
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

	return info.PublicKey(), nil
}

// NewKeyInfo creates a new KeyInfo struct in the wallet backend and returns it
func (w *Wallet) NewKeyInfo() (*types.KeyInfo, error) {
	newAddr, err := NewAddress(w)
	if err != nil {
		return &types.KeyInfo{}, err
	}

	return w.keyInfoForAddr(newAddr)
}

// CreateTicket computes a valid ticket.
// 	params:  proof  []byte, minerAddress address.Address.
//  returns:  types.Signature ( []byte ), error
func (w *Wallet) CreateTicket(proof proofs.PoStProof, signerPubKey []byte) (types.Signature, error) {

	var ticket types.Signature

	signerAddr, err := w.GetAddressForPubKey(signerPubKey)
	if err != nil {
		msgString := fmt.Sprintf("addresses: %v", w.Addresses())
		return ticket, errors.Wrap(err, msgString)
	}

	buf := append(proof[:], signerAddr.Bytes()...)
	h := blake2b.Sum256(buf)

	ticket, err = w.SignBytes(h[:], signerAddr)
	if err != nil {
		errMsg := fmt.Sprintf("SignBytes error in CreateTicket: %s", err.Error())
		return ticket, errors.New(errMsg)
	}
	return ticket, nil
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
