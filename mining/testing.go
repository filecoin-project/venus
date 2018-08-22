package mining

import (
	"context"
	"sync"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// BlockTimeTest is the block time used by workers during testing
const BlockTimeTest = time.Millisecond * 300

// MineDelayTest is the mining delay used by schedulers during testing
const MineDelayTest = time.Millisecond * 10

// MockScheduler is a mock Scheduler.
type MockScheduler struct {
	mock.Mock
}

// Start is the MockScheduler's Start function.
func (s *MockScheduler) Start(ctx context.Context) (chan<- Input, <-chan Output, *sync.WaitGroup) {
	args := s.Called(ctx)
	return args.Get(0).(chan<- Input), args.Get(1).(<-chan Output), args.Get(2).(*sync.WaitGroup)
}

// TestWorker is a worker with a customizable work function to facilitate
// easy testing.
type TestWorker struct {
	WorkFunc func(context.Context, Input, chan<- Output)
}

// Mine is the TestWorker's Work function.  It simply calls the WorkFunc
// field.
func (w *TestWorker) Mine(ctx context.Context, input Input, outCh chan<- Output) {
	if w.WorkFunc == nil {
		panic("must set MutableTestWorker's WorkFunc before calling Work")
	}
	w.WorkFunc(ctx, input, outCh)
}

// NewTestWorkerWithDeps creates a worker that calls the provided input
// function when Mine() is called.
func NewTestWorkerWithDeps(f func(context.Context, Input, chan<- Output)) *TestWorker {
	return &TestWorker{
		WorkFunc: f,
	}
}

// MakeEchoMine returns a test worker function that itself returns the first
// block of the input tipset as output.
func MakeEchoMine(require *require.Assertions) func(context.Context, Input, chan<- Output) {
	echoMine := func(c context.Context, i Input, outCh chan<- Output) {
		require.NotEqual(0, len(i.TipSet))
		b := i.TipSet.ToSlice()[0]
		outCh <- Output{NewBlock: b}
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
func ReceiveInCh(ch <-chan Input) int {
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
