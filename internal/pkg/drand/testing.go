package drand

import (
	"context"
	"encoding/binary"
	"time"

	ffi "github.com/filecoin-project/filecoin-ffi"
)

const testDRANDRoundDuration = 25 * time.Second

// Fake is a fake drand utility that reads and validates entries as specified below
type Fake struct {
	// Time of round 0
	GenesisTime   time.Time
	FirstFilecoin Round
}

var _ IFace = &Fake{}

// NewFake sets up a fake drand that starts exactly one testDRANDRoundDuration before
// the provided filecoin genesis time.
func NewFake(filecoinGenTime time.Time) *Fake {
	drandGenTime := filecoinGenTime.Add(-1 * testDRANDRoundDuration)
	return &Fake{
		GenesisTime:   drandGenTime,
		FirstFilecoin: Round(0),
	}
}

// ReadEntry immediately returns a drand entry with a signature equal to the
// round number
func (d *Fake) ReadEntry(_ context.Context, drandRound Round) (*Entry, error) {
	fakeSigData := make([]byte, ffi.SignatureBytes)
	binary.PutUvarint(fakeSigData, uint64(drandRound))
	return &Entry{
		Round: drandRound,
		Data:  fakeSigData,
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
func (d *Fake) RoundsInInterval(startTime, endTime time.Time) []Round {
	return roundsInInterval(startTime, endTime, d.StartTimeOfRound, testDRANDRoundDuration)
}

func (d *Fake) FirstFilecoinRound() Round {
	return d.FirstFilecoin
}

// FetchGroupConfig returns empty group addresses and key coefficients
func (d *Fake) FetchGroupConfig(_ []string, _, _ bool) ([]string, [][]byte, uint64, int, error) {
	return []string{}, [][]byte{}, 0, 0, nil
}
