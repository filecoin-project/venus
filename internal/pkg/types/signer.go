package types

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/crypto"
)

// Signer is an interface for SignBytes
type Signer interface {
	SignBytes(data []byte, addr address.Address) (crypto.Signature, error)
}
