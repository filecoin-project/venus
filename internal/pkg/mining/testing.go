package mining

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MineDelayTest is the mining delay used by schedulers during testing
const MineDelayTest = time.Millisecond * 500

// MockScheduler is a mock Scheduler.
type MockScheduler struct {
	mock.Mock
	isStarted bool
}

// Start is the MockScheduler's Start function.
func (s *MockScheduler) Start(ctx context.Context) (<-chan Output, *sync.WaitGroup) {
	args := s.Called(ctx)
	s.isStarted = true
	return args.Get(0).(<-chan Output), args.Get(1).(*sync.WaitGroup)
}

// IsStarted is the equivalent to scheduler.IsStarted, to know when the scheduler
// has been started.
func (s *MockScheduler) IsStarted() bool {
	return s.isStarted
}

// TestWorker is a worker with a customizable work function to facilitate
// easy testing.
type TestWorker struct {
	WorkFunc func(context.Context, block.TipSet, uint64, chan<- Output) bool
}

// Mine is the TestWorker's Work function.  It simply calls the WorkFunc
// field.
func (w *TestWorker) Mine(ctx context.Context, ts block.TipSet, nullBlockCount uint64, outCh chan<- Output) bool {
	if w.WorkFunc == nil {
		panic("must set MutableTestWorker's WorkFunc before calling Work")
	}
	return w.WorkFunc(ctx, ts, nullBlockCount, outCh)
}

// NewTestWorkerWithDeps creates a worker that calls the provided input
// function when Mine() is called.
func NewTestWorkerWithDeps(f func(context.Context, block.TipSet, uint64, chan<- Output) bool) *TestWorker {
	return &TestWorker{
		WorkFunc: f,
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

const (
	// ChannelClosed is returned by the Receive*Ch helper functions to indicate
	// the cahnnel is closed.
	ChannelClosed = iota
	// ChannelEmpty indicates an empty channel.
	ChannelEmpty
	// ChannelReceivedValue indicates the channel held a value, which has been
	// received.
	ChannelReceivedValue
)

// ReceiveInCh returns the channel status.
/*func ReceiveInCh(ch <-chan Input) int {
	select {
	case _, ok := <-ch:
		if ok {
			return ChannelReceivedValue
		}
		return ChannelClosed
	default:
		return ChannelEmpty
	}
}*/

// ReceiveOutCh returns the channel status.
func ReceiveOutCh(ch <-chan Output) int {
	select {
	case _, ok := <-ch:
		if ok {
			return ChannelReceivedValue
		}
		return ChannelClosed
	default:
		return ChannelEmpty
	}
}

// NthTicket returns a ticket with a vrf proof equal to a byte slice wrapping
// the input uint8 value.
func NthTicket(i uint8) block.Ticket {
	return block.Ticket{VRFProof: []byte{i}}
}
