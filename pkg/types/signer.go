package types

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/crypto"
)

// Signer signs data with a private key obtained internally from a provided address.
type Signer interface {
	SignBytes(ctx context.Context, data []byte, addr address.Address) (crypto.Signature, error)
	HasAddress(ctx context.Context, addr address.Address) (bool, error)
}
