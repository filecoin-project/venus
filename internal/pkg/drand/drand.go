package drand

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
)

// IFace is the standard inferface for interacting with the drand network
type IFace interface {
	ReadEntry(ctx context.Context, drandRound Round) (*Entry, error)
	VerifyEntry(parent, child *Entry) (bool, error)
}

// Utility reads drand entries and verifies drand entries against their parents
type Utility struct{}

// ReadEntry returns the drand entry at drandRound.  It should block on reading
// from the network in case this drandRound entry has not been propagated yet.
func (d *Utility) ReadEntry(ctx context.Context, drandRound Round) (*Entry, error) {
	panic("TODO: this is a stub that needs to be filled in")
}

// VerifyEntry returns true if the child entry was signed correctly off of the
// parent entry per the drand protocol.
func (d *Utility) VerifyEntry(parent, child *Entry) (bool, error) {
	panic("TODO: this is a stub that needs to be filled in")
}

// Round is a type for recording drand round indexes
type Round uint64

// Entry is a verifiable entry in the drand chain carrying round and
// randomness information
type Entry struct {
	_         struct{} `cbor:",toarray"`
	Round     Round
	Signature crypto.Signature
}
