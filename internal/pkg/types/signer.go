package types

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/crypto"
)

// Signer signs data with a private key obtained internally from a provided address.
type Signer interface {
	SignBytes(data []byte, addr address.Address) (crypto.Signature, error)
}
