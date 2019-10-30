package types

import "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"

// Signer is an interface for SignBytes
type Signer interface {
	SignBytes(data []byte, addr address.Address) (Signature, error)
}
