package mining

import (
	"context"
	"sync"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

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
	WorkFun func(context.Context, Input, chan<- Output)
}

// Mine is the TestWorker's Work function.  It simply calls the workFun
// field.
func (w *TestWorker) Mine(ctx context.Context, input Input, outCh chan<- Output) {
	if w.WorkFun == nil {
		panic("must set MutableTestWorker's WorkFun before calling Work")
	}
	w.WorkFun(ctx, input, outCh)
}

func newTestWorkerWithDeps(f func(context.Context, Input, chan<- Output)) *TestWorker {
	return &TestWorker{
		WorkFun: f,
	}
}

func makeEchoMine(require *require.Assertions) func(context.Context, Input, chan<- Output) {
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
