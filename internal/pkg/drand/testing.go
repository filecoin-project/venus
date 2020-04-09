package drand

import (
	"context"
	"encoding/binary"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"

	ffi "github.com/filecoin-project/filecoin-ffi"
)

const testDRANDRoundDuration = 25 * time.Second

// Fake is a fake drand utility that reads and validates entries as specified below
type Fake struct {
	// Time of round 0
	GenesisTime   time.Time
	FirstFilecoin Round
}

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

func (d *Fake) StartTimeOfRound(round Round) time.Time {
	return d.GenesisTime.Add(testDRANDRoundDuration * time.Duration(round))
}

// RoundsInInterval returns the DRAND round numbers within [startTime, endTime)
// startTime inclusive, endTime exclusive.
// No gaps in test DRAND so this doesn't need to consult the DRAND chain
func (d *Fake) RoundsInInterval(startTime, endTime time.Time) []Round {
	// Find first round after startTime
	truncatedStartRound := Round(startTime.Sub(d.GenesisTime) / testDRANDRoundDuration)
	var round Round
	if d.StartTimeOfRound(truncatedStartRound).Equal(startTime) {
		round = truncatedStartRound
	} else {
		round = truncatedStartRound + 1
	}
	roundTime := d.StartTimeOfRound(round)
	var rounds []Round
	// Advance a round time until we hit endTime, adding rounds
	for roundTime.Before(endTime) {
		rounds = append(rounds, round)
		round++
		roundTime = d.StartTimeOfRound(round)
	}
	return rounds
}

func (d *Fake) FirstFilecoinRound() Round {
	return d.FirstFilecoin
}
