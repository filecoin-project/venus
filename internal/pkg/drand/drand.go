package drand

import (
	"context"
	"time"
)

// IFace is the standard inferface for interacting with the drand network
type IFace interface {
	ReadEntry(ctx context.Context, drandRound Round) (*Entry, error)
	VerifyEntry(parent, child *Entry) (bool, error)
	FetchGroupConfig(addresses []string, secure bool, overrideGroupAddrs bool) ([]string, [][]byte, uint64, int, error)
	StartTimeOfRound(round Round) time.Time
	RoundsInInterval(startTime, endTime time.Time) []Round
	FirstFilecoinRound() Round
}

// Round is a type for recording drand round indexes
type Round uint64

// A verifiable entry from a beacon chain, carrying round and randomness information.
type Entry struct {
	_     struct{} `cbor:",toarray"`
	Round Round
	Data  []byte
}
