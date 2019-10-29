package wallet

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

// Backend is the interface to represent different storage backends
// that can contain many addresses.
type Backend interface {
	// Addresses returns a list of all accounts currently stored in this backend.
	Addresses() []address.Address

	// Contains returns true if this backend stores the passed in address.
	HasAddress(addr address.Address) bool

	// Sign cryptographically signs `data` using the private key `priv`.
	SignBytes(data []byte, addr address.Address) (types.Signature, error)

	// GetKeyInfo will return the keyinfo associated with address `addr`
	// iff backend contains the addr.
	GetKeyInfo(addr address.Address) (*types.KeyInfo, error)
}

// Importer is a specialization of a wallet backend that can import
// new keys into its permanent storage. Disk backed wallets can do this,
// hardware wallets generally cannot.
type Importer interface {
	// ImportKey imports the key described by the given keyinfo
	// into the backend
	ImportKey(ki *types.KeyInfo) error
}
