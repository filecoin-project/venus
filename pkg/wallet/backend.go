package wallet

import (
	"context"

	"github.com/filecoin-project/go-address"

	"github.com/filecoin-project/venus/pkg/crypto"
)

// Backend is the interface to represent different storage backends
// that can contain many addresses.
type Backend interface {
	// Addresses returns a list of all accounts currently stored in this backend.
	Addresses(ctx context.Context) []address.Address

	// Contains returns true if this backend stores the passed in address.
	HasAddress(context.Context, address.Address) bool

	// Sign cryptographically signs data with the private key associated with an address.
	SignBytes(context.Context, []byte, address.Address) (*crypto.Signature, error)

	// GetKeyInfo will return the keyinfo associated with address `addr`
	// iff backend contains the addr.
	GetKeyInfo(context.Context, address.Address) (*crypto.KeyInfo, error)

	GetKeyInfoPassphrase(context.Context, address.Address, []byte) (*crypto.KeyInfo, error)

	LockWallet(context.Context) error
	UnLockWallet(context.Context, []byte) error
	WalletState(context.Context) int
}

// Importer is a specialization of a wallet backend that can import
// new keys into its permanent storage. Disk backed wallets can do this,
// hardware wallets generally cannot.
type Importer interface {
	// ImportKey imports the key described by the given keyinfo
	// into the backend
	ImportKey(context.Context, *crypto.KeyInfo) error
}
