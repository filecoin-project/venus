package wallet

import "github.com/filecoin-project/go-filecoin/types"

// Backend is the interface to represent different storage backends
// that can contain many addresses.
type Backend interface {
	// Addresses returns a list of all accounts currently stored in this backend.
	Addresses() []types.Address

	// Contains returns true if this backend stores the passed in address.
	HasAddress(addr types.Address) bool
}
