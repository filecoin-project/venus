package wallet

import (
	"crypto/ecdsa"
	"github.com/filecoin-project/go-filecoin/types"
)

// Backend is the interface to represent different storage backends
// that can contain many addresses.
type Backend interface {
	// Addresses returns a list of all accounts currently stored in this backend.
	Addresses() []types.Address

	// Contains returns true if this backend stores the passed in address.
	HasAddress(addr types.Address) bool

	// Sign cryptographically signs `data` using the private key `priv`.
	SignBytes(data []byte, addr types.Address) (types.Signature, error)

	// Verify cryptographically verifies that 'sig' is the signed hash of 'data' with
	// the public key `pk`.
	Verify(data []byte, pk []byte, sig types.Signature) (bool, error)

	// Ecrecover returns an uncompressed public key that could produce the given
	// signature from data.
	// Note: The returned public key should not be used to verify `data` is valid
	// since a public key may have N private key pairs
	Ecrecover(data []byte, sig types.Signature) ([]byte, error)

	// GetKeyPair will return the private & public keys associated with address `addr`
	// iff backend contains the addr.
	GetKeyPair(addr types.Address) (*ecdsa.PrivateKey, *ecdsa.PublicKey, error)
}
