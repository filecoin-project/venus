package wallet

import (
	"crypto/ecdsa"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"sync"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmWHbPAp5UWfwZE3XCgD93xsCYZyk12tAAQVL3QXLKcWaj/toml"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/crypto"
	cu "github.com/filecoin-project/go-filecoin/crypto/util"
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

// File holds the values representing a wallet when writing to a file.
type File struct {
	AddressKeyPairs []AddressKeyInfo `toml:"wallet"`
}

// AddressKeyInfo holds the address and KeyInfo used to generate it.
type AddressKeyInfo struct {
	AddressInfo AddressInfo   `toml:"addressinfo"`
	KeyInfo     types.KeyInfo `toml:"keyinfo"`
}

// AddressInfo holds an address and a balance.
type AddressInfo struct {
	Address string `toml:"address"`
	Balance uint64 `toml:"balance"`
}

// TypesAddressInfo makes it easier for the receiever to deal with types already made
type TypesAddressInfo struct {
	Address address.Address
	Balance *types.AttoFIL
}

// LoadWalletAddressAndKeysFromFile will load the addresses and their keys from File
// `file`. The balance field may also be set to specify what an address's balance
// should be at time of genesis.
func LoadWalletAddressAndKeysFromFile(file string) (map[TypesAddressInfo]types.KeyInfo, error) {
	wf, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read wallet file")
	}

	wt := new(File)
	if err = toml.Unmarshal(wf, &wt); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal wallet file")
	}

	if len(wt.AddressKeyPairs) == 0 {
		return nil, errors.New("wallet file does not contain any addresses")
	}

	loadedAddrs := make(map[TypesAddressInfo]types.KeyInfo)
	for _, akp := range wt.AddressKeyPairs {
		addr, err := address.NewFromString(akp.AddressInfo.Address)
		if err != nil {
			return nil, err
		}
		tai := TypesAddressInfo{
			Address: addr,
			Balance: types.NewAttoFILFromFIL(akp.AddressInfo.Balance),
		}
		loadedAddrs[tai] = akp.KeyInfo
	}
	return loadedAddrs, nil
}

// GenerateWalletFile will generate a WalletFile with `numAddrs` addresses
func GenerateWalletFile(numAddrs int) (*File, error) {
	var wt File
	for i := 0; i < numAddrs; i++ {
		prv, err := crypto.GenerateKey()
		if err != nil {
			return nil, err
		}

		pub, ok := prv.Public().(*ecdsa.PublicKey)
		if !ok {
			// means a something is wrong with key generation
			panic("unknown public key type")
		}

		addrHash := address.Hash(cu.SerializeUncompressed(pub))
		// TODO: Use the address type we are running on from the config.
		newAddr := address.NewMainnet(addrHash)

		ki := &types.KeyInfo{
			PrivateKey: crypto.ECDSAToBytes(prv),
			Curve:      SECP256K1,
		}

		aki := AddressKeyInfo{
			AddressInfo: AddressInfo{
				Address: newAddr.String(),
				Balance: 10000000,
			},
			KeyInfo: *ki,
		}
		wt.AddressKeyPairs = append(wt.AddressKeyPairs, aki)
	}
	return &wt, nil
}

// WriteWalletFile will write WalletFile `wf` to path `file`
func WriteWalletFile(file string, wf File) error {
	f, err := os.OpenFile(file, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return errors.Wrap(err, "failed to open wallet file for writing")
	}
	defer f.Close() // nolint: errcheck

	if err := toml.NewEncoder(f).Encode(wf); err != nil {
		return errors.Wrap(err, "faild to encode wallet")
	}
	return nil
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
