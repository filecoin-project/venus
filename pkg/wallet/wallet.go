package wallet

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"sort"
	"sync"

	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/filecoin-project/go-address"
	logging "github.com/ipfs/go-log/v2"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/pkg/crypto"
)

var TestPassword = []byte("test-password")

var ErrKeyInfoNotFound = fmt.Errorf("key info not found")
var walletLog = logging.Logger("wallet")

// WalletIntersection
// nolint
type WalletIntersection interface {
	HasAddress(ctx context.Context, a address.Address) bool
	Addresses(ctx context.Context) []address.Address
	NewAddress(ctx context.Context, p address.Protocol) (address.Address, error)
	Import(ctx context.Context, ki *crypto.KeyInfo) (address.Address, error)
	Export(ctx context.Context, addr address.Address, password string) (*crypto.KeyInfo, error)
	WalletSign(ctx context.Context, keyAddr address.Address, msg []byte, meta types.MsgMeta) (*crypto.Signature, error)
	HasPassword(ctx context.Context) bool
}

var _ WalletIntersection = &Wallet{}

// wallet manages the locally stored addresses.
type Wallet struct {
	lk sync.Mutex

	backends map[reflect.Type][]Backend
}

// New constructs a new wallet, that manages addresses in all the
// passed in backends.
func New(backends ...Backend) *Wallet {
	backendsMap := make(map[reflect.Type][]Backend)

	for _, backend := range backends {
		typ := reflect.TypeOf(backend)
		backendsMap[typ] = append(backendsMap[typ], backend)
	}

	return &Wallet{
		backends: backendsMap,
	}
}

// HasAddress checks if the given address is stored.
// Safe for concurrent access.
func (w *Wallet) HasAddress(ctx context.Context, a address.Address) bool {
	_, err := w.Find(ctx, a)
	return err == nil
}

// Find searches through all backends and returns the one storing the passed
// in address.
// Safe for concurrent access.
func (w *Wallet) Find(ctx context.Context, addr address.Address) (Backend, error) {
	w.lk.Lock()
	defer w.lk.Unlock()

	for _, backends := range w.backends {
		for _, backend := range backends {
			if backend.HasAddress(ctx, addr) {
				return backend, nil
			}
		}
	}

	return nil, fmt.Errorf("wallet has no address %s", addr)
}

// Addresses retrieves all stored addresses.
// Safe for concurrent access.
// Always sorted in the same order.
func (w *Wallet) Addresses(ctx context.Context) []address.Address {
	w.lk.Lock()
	defer w.lk.Unlock()

	var out []address.Address
	for _, backends := range w.backends {
		for _, backend := range backends {
			out = append(out, backend.Addresses(ctx)...)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return bytes.Compare(out[i].Bytes(), out[j].Bytes()) < 0
	})

	return out
}

// Backends returns backends by their type.
func (w *Wallet) Backends(typ reflect.Type) []Backend {
	w.lk.Lock()
	defer w.lk.Unlock()

	cpy := make([]Backend, len(w.backends[typ]))
	copy(cpy, w.backends[typ])
	return cpy
}

// SignBytes cryptographically signs `data` using the private key corresponding to
// address `addr`
func (w *Wallet) SignBytes(ctx context.Context, data []byte, addr address.Address) (*crypto.Signature, error) {
	// Check that we are storing the address to sign for.
	backend, err := w.Find(ctx, addr)
	if err != nil {
		return nil, errors.Wrapf(err, "could not find address: %s", addr)
	}
	return backend.SignBytes(ctx, data, addr)
}

// NewAddress creates a new account address on the default wallet backend.
func (w *Wallet) NewAddress(ctx context.Context, p address.Protocol) (address.Address, error) {
	backend, err := w.DSBacked()
	if err != nil {
		return address.Undef, err
	}
	return backend.NewAddress(ctx, p)
}

// GetPubKeyForAddress returns the public key in the keystore associated with
// the given address.
func (w *Wallet) GetPubKeyForAddress(ctx context.Context, addr address.Address) ([]byte, error) {
	info, err := w.keyInfoForAddr(ctx, addr)
	if err != nil {
		return nil, err
	}

	return info.PublicKey()
}

// NewKeyInfo creates a new KeyInfo struct in the wallet backend and returns it
func (w *Wallet) NewKeyInfo(ctx context.Context) (*crypto.KeyInfo, error) {
	newAddr, err := w.NewAddress(ctx, address.BLS)
	if err != nil {
		return &crypto.KeyInfo{}, err
	}

	return w.keyInfoForAddr(ctx, newAddr)
}

func (w *Wallet) keyInfoForAddr(ctx context.Context, addr address.Address) (*crypto.KeyInfo, error) {
	backend, err := w.Find(ctx, addr)
	if err != nil {
		return &crypto.KeyInfo{}, err
	}

	info, err := backend.GetKeyInfo(ctx, addr)
	if err != nil {
		return &crypto.KeyInfo{}, err
	}
	return info, nil
}

// Import adds the given keyinfo to the wallet
func (w *Wallet) Import(ctx context.Context, ki *crypto.KeyInfo) (address.Address, error) {
	dsb := w.Backends(DSBackendType)
	if len(dsb) != 1 {
		return address.Undef, fmt.Errorf("expected exactly one datastore wallet backend")
	}

	imp, ok := dsb[0].(Importer)
	if !ok {
		return address.Undef, fmt.Errorf("datastore backend wallets should implement importer")
	}

	if err := imp.ImportKey(ctx, ki); err != nil {
		return address.Undef, err
	}

	a, err := ki.Address()
	if err != nil {
		return address.Undef, err
	}
	return a, nil
}

// Export returns the KeyInfos for the given wallet addresses
func (w *Wallet) Export(ctx context.Context, addr address.Address, password string) (*crypto.KeyInfo, error) {
	bck, err := w.Find(ctx, addr)
	if err != nil {
		return nil, err
	}

	ki, err := bck.GetKeyInfoPassphrase(ctx, addr, []byte(password))
	if err != nil {
		return nil, err
	}

	return ki, nil
}

//WalletSign used to sign message with private key
func (w *Wallet) WalletSign(ctx context.Context, addr address.Address, msg []byte, meta types.MsgMeta) (*crypto.Signature, error) {
	ki, err := w.Find(ctx, addr)
	if err != nil {
		return nil, err
	}
	if ki == nil {
		return nil, errors.Errorf("signing using key '%s': %v", addr.String(), ErrKeyInfoNotFound)
	}

	return ki.SignBytes(ctx, msg, addr)
}

//DSBacked return the first wallet backend
//todo support multi wallet backend
func (w *Wallet) DSBacked() (*DSBackend, error) {
	backends := w.Backends(DSBackendType)
	if len(backends) == 0 {
		return nil, errors.Errorf("missing default ds backend")
	}

	return (backends[0]).(*DSBackend), nil
}

//LockWallet lock lock wallet
func (w *Wallet) LockWallet(ctx context.Context) error {
	backend, err := w.DSBacked()
	if err != nil {
		return err
	}

	return backend.LockWallet(ctx)
}

//UnLockWallet unlock local wallet with password

func (w *Wallet) UnLockWallet(ctx context.Context, password []byte) error {
	backend, err := w.DSBacked()
	if err != nil {
		return err
	}
	return backend.UnLockWallet(ctx, password)
}

//SetPassword
func (w *Wallet) SetPassword(ctx context.Context, password []byte) error {
	backend, err := w.DSBacked()
	if err != nil {
		return err
	}
	return backend.SetPassword(ctx, password)
}

//HasPassword return whether the password has been set in the wallet
func (w *Wallet) HasPassword(ctx context.Context) bool {
	backend, err := w.DSBacked()
	if err != nil {
		walletLog.Errorf("get DSBacked failed: %v", err)
		return false
	}
	return backend.HasPassword()
}

//WalletState return wallet state(lock/unlock)
func (w *Wallet) WalletState(ctx context.Context) int {
	backend, err := w.DSBacked()
	if err != nil {
		walletLog.Errorf("get DSBacked failed: %v", err)
		return undetermined
	}
	return backend.WalletState(ctx)
}
