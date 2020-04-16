package drand

import (
	"context"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
)

// IFace is the standard inferface for interacting with the drand network
type IFace interface {
	ReadEntry(ctx context.Context, drandRound Round) (*Entry, error)
	VerifyEntry(parent, child *Entry) (bool, error)
	FetchGroupConfig(addresses []string, secure bool, overrideGroupAddrs bool) ([]string, [][]byte, error)
	StartTimeOfRound(round Round) time.Time
	RoundsInInterval(startTime, endTime time.Time) []Round
	FirstFilecoinRound() Round
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
