package drand

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
)

// IFace is the standard inferface for interacting with the drand network
type IFace interface {
	ReadEntry(ctx context.Context, drandRound Round) (*Entry, error)
	VerifyEntry(parent, child *Entry) (bool, error)
	FetchGroupConfig(addresses []string, secure bool, overrideGroupAddrs bool) ([]string, []string, error)
}

// Round is a type for recording drand round indexes
type Round uint64

// Entry is a verifiable entry in the drand chain carrying round and
// randomness information
type Entry struct {
	Round     Round
	Signature crypto.Signature
}
