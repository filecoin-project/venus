package mining

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

type miningFunc func(runCtx context.Context, base block.TipSet, nullBlkCount uint64) (*FullBlock, error)

// TestWorker is a worker with a customizable work function to facilitate
// easy testing.
type TestWorker struct {
	WorkFunc miningFunc
	t        *testing.T
}

// Mine is the TestWorker's Work function.  It simply calls the WorkFunc
// field.
func (w *TestWorker) Mine(ctx context.Context, ts block.TipSet, nullBlockCount uint64) (*FullBlock, error) {
	require.NotNil(w.t, w.WorkFunc)
	return w.WorkFunc(ctx, ts, nullBlockCount)
}

// NewTestWorker creates a worker that calls the provided input
// function when Mine() is called.
func NewTestWorker(t *testing.T, f miningFunc) *TestWorker {
	return &TestWorker{
		WorkFunc: f,
		t:        t,
	}
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
