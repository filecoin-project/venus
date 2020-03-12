package mining

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/stretchr/testify/require"
)

// TestWorker is a worker with a customizable work function to facilitate
// easy testing.
type TestWorker struct {
	WorkFunc func(context.Context, block.TipSet, uint64, chan<- Output) bool
	t        *testing.T
}

// Mine is the TestWorker's Work function.  It simply calls the WorkFunc
// field.
func (w *TestWorker) Mine(ctx context.Context, ts block.TipSet, nullBlockCount uint64, outCh chan<- Output) bool {
	require.NotNil(w.t, w.WorkFunc)
	return w.WorkFunc(ctx, ts, nullBlockCount, outCh)
}

// NewTestWorker creates a worker that calls the provided input
// function when Mine() is called.
func NewTestWorker(t *testing.T, f func(context.Context, block.TipSet, uint64, chan<- Output) bool) *TestWorker {
	return &TestWorker{
		WorkFunc: f,
		t:        t,
	}
}

// MakeEchoMine returns a test worker function that itself returns the first
// block of the input tipset as output.
func MakeEchoMine(t *testing.T) func(context.Context, block.TipSet, uint64, chan<- Output) bool {
	echoMine := func(c context.Context, ts block.TipSet, nullBlockCount uint64, outCh chan<- Output) bool {
		require.True(t, ts.Defined())
		b := ts.At(0)
		select {
		case outCh <- Output{NewBlock: b}:
		case <-c.Done():
		}
		return true
	}
	return echoMine
}

// NthTicket returns a ticket with a vrf proof equal to a byte slice wrapping
// the input uint8 value.
func NthTicket(i uint8) block.Ticket {
	return block.Ticket{VRFProof: []byte{i}}
}

// NoMessageQualifier always returns no error
type NoMessageQualifier struct{}

func (npc *NoMessageQualifier) PenaltyCheck(_ context.Context, _ *types.UnsignedMessage) error {
	return nil
}
