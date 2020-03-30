package drand

import (
	"context"
	"encoding/binary"

	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"

	ffi "github.com/filecoin-project/filecoin-ffi"
)

// Fake is a fake drand utility that reads and validates entries as specified below
type Fake struct{}

// ReadEntry immediately returns a drand entry with a signature equal to the
// round number
func (d *Fake) ReadEntry(ctx context.Context, drandRound Round) (*Entry, error) {
	fakeSigData := make([]byte, ffi.SignatureBytes)
	binary.PutUvarint(fakeSigData, uint64(drandRound))
	return &Entry{
		Round: drandRound,
		Signature: crypto.Signature{
			Type: crypto.SigTypeBLS,
			Data: fakeSigData,
		},
	}, nil
}

// VerifyEntry always returns true without error
func (d *Fake) VerifyEntry(parent, child *Entry) (bool, error) {
	return true, nil
}
