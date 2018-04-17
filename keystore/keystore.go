package keystore

import (
	"fmt"

	ci "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
)

// Keystore provides a key management interface
type Keystore interface {
	// Has returns whether or not a key exist in the Keystore
	Has(string) (bool, error)
	// Put stores a key in the Keystore, if a key with the same name already exists, returns ErrKeyExists
	Put(string, ci.PrivKey) error
	// Get retrieves a key from the Keystore if it exists, and returns ErrNoSuchKey
	// otherwise.
	Get(string) (ci.PrivKey, error)
	// Delete removes a key from the Keystore
	Delete(string) error
	// List returns a list of key identifier
	List() ([]string, error)
}

var ErrNoSuchKey = fmt.Errorf("no key by the given name was found")                     // nolint: golint
var ErrKeyExists = fmt.Errorf("key by that name already exists, refusing to overwrite") // nolint: golint
var ErrKeyFmt = fmt.Errorf("Key has invalid format")                                    // nolint: golint
