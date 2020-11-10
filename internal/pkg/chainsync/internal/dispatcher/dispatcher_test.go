package dispatcher_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/chainsync/internal/dispatcher"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/internal/pkg/types"
	"github.com/filecoin-project/venus/internal/pkg/util/moresync"
)

type mockSyncer struct {
	headsCalled []block.TipSetKey
}

type noopTransitioner struct{}

func (nt *noopTransitioner) MaybeTransitionToCatchup(inCatchup bool, _ []dispatcher.Target) (bool, error) {
	return inCatchup, nil
}
func (nt *noopTransitioner) MaybeTransitionToFollow(_ context.Context, inCatchup bool, _ int) (bool, error) {
	return !inCatchup, nil
}
func (nt *noopTransitioner) TransitionChannel() chan bool {
	return nil
}

func (fs *mockSyncer) HandleNewTipSet(_ context.Context, ci *block.ChainInfo, _ bool) error {
	fs.headsCalled = append(fs.headsCalled, ci.Head)
	return nil
}

func TestDispatchStartHappy(t *testing.T) {
	tf.UnitTest(t)
	s := &mockSyncer{
		headsCalled: make([]block.TipSetKey, 0),
	}
	nt := &noopTransitioner{}
	testDispatch := dispatcher.NewDispatcher(s, nt)

	cis := []*block.ChainInfo{
		// We need to put these in priority order to avoid a race.
		// If we send 0 before 42, it is possible the dispatcher will
		// pick up 0 and start processing before it sees 42.
		chainInfoFromHeight(t, 42),
		chainInfoFromHeight(t, 16),
		chainInfoFromHeight(t, 3),
		chainInfoFromHeight(t, 2),
		chainInfoFromHeight(t, 0),
	}

	testDispatch.Start(context.Background())

	// set up a blocking channel and register to unblock after 5 synced
	allDone := moresync.NewLatch(5)
	testDispatch.RegisterCallback(func(t dispatcher.Target, _ error) { allDone.Done() })

	// receive requests before Start() to test deterministic order
	go func() {
		for _, ci := range cis {
			assert.NoError(t, testDispatch.SendHello(ci))
		}
	}()
	allDone.Wait()

	// check that the mockSyncer synced in order
	require.Equal(t, 5, len(s.headsCalled))
	for i := range cis {
		assert.Equal(t, cis[i].Head, s.headsCalled[i])
	}
}

func TestDispatcherDropsWhenFull(t *testing.T) {
	tf.UnitTest(t)
	s := &mockSyncer{
		headsCalled: make([]block.TipSetKey, 0),
	}
	nt := &noopTransitioner{}
	testWorkSize := 20
	testBufferSize := 30
	testDispatch := dispatcher.NewDispatcherWithSizes(s, nt, testWorkSize, testBufferSize)

	finished := moresync.NewLatch(1)
	testDispatch.RegisterCallback(func(target dispatcher.Target, _ error) {
		// Fail if the work that should be dropped gets processed
		assert.False(t, target.Height == 100)
		assert.False(t, target.Height == 101)
		assert.False(t, target.Height == 102)
		if target.Height == 0 {
			// 0 has lowest priority of non-dropped
			finished.Done()
		}
	})
	for j := 0; j < testWorkSize; j++ {
		ci := chainInfoFromHeight(t, j)
		assert.NoError(t, testDispatch.SendHello(ci))
	}
	// Should be dropped
	assert.NoError(t, testDispatch.SendHello(chainInfoFromHeight(t, 100)))
	assert.NoError(t, testDispatch.SendHello(chainInfoFromHeight(t, 101)))
	assert.NoError(t, testDispatch.SendHello(chainInfoFromHeight(t, 102)))

	testDispatch.Start(context.Background())

	finished.Wait()
}

func TestQueueHappy(t *testing.T) {
	tf.UnitTest(t)
	testQ := dispatcher.NewTargetQueue()

	// Add syncRequests out of order
	sR0 := dispatcher.Target{ChainInfo: *(chainInfoFromHeight(t, 0))}
	sR1 := dispatcher.Target{ChainInfo: *(chainInfoFromHeight(t, 1))}
	sR2 := dispatcher.Target{ChainInfo: *(chainInfoFromHeight(t, 2))}
	sR47 := dispatcher.Target{ChainInfo: *(chainInfoFromHeight(t, 47))}

	testQ.Push(sR2)
	testQ.Push(sR47)
	testQ.Push(sR0)
	testQ.Push(sR1)

	assert.Equal(t, 4, testQ.Len())

	// Pop in order
	out0 := requirePop(t, testQ)
	out1 := requirePop(t, testQ)
	out2 := requirePop(t, testQ)
	out3 := requirePop(t, testQ)

	assert.Equal(t, abi.ChainEpoch(47), out0.ChainInfo.Height)
	assert.Equal(t, abi.ChainEpoch(2), out1.ChainInfo.Height)
	assert.Equal(t, abi.ChainEpoch(1), out2.ChainInfo.Height)
	assert.Equal(t, abi.ChainEpoch(0), out3.ChainInfo.Height)

	assert.Equal(t, 0, testQ.Len())
}

func TestQueueDuplicates(t *testing.T) {
	tf.UnitTest(t)
	testQ := dispatcher.NewTargetQueue()

	// Add syncRequests with same height
	sR0 := dispatcher.Target{ChainInfo: *(chainInfoFromHeight(t, 0))}
	sR0dup := dispatcher.Target{ChainInfo: *(chainInfoFromHeight(t, 0))}

	testQ.Push(sR0)
	testQ.Push(sR0dup)

	// Only one of these makes it onto the queue
	assert.Equal(t, 1, testQ.Len())

	// Pop
	first := requirePop(t, testQ)
	assert.Equal(t, abi.ChainEpoch(0), first.ChainInfo.Height)

	// Now if we push the duplicate it goes back on
	testQ.Push(sR0dup)
	assert.Equal(t, 1, testQ.Len())

	second := requirePop(t, testQ)
	assert.Equal(t, abi.ChainEpoch(0), second.ChainInfo.Height)
}

func TestQueueEmptyPopErrors(t *testing.T) {
	tf.UnitTest(t)
	testQ := dispatcher.NewTargetQueue()
	sR0 := dispatcher.Target{ChainInfo: *(chainInfoFromHeight(t, 0))}
	sR47 := dispatcher.Target{ChainInfo: *(chainInfoFromHeight(t, 47))}

	// Push 2
	testQ.Push(sR47)
	testQ.Push(sR0)

	// Pop 3
	assert.Equal(t, 2, testQ.Len())
	_ = requirePop(t, testQ)
	assert.Equal(t, 1, testQ.Len())
	_ = requirePop(t, testQ)
	assert.Equal(t, 0, testQ.Len())

	_, popped := testQ.Pop()
	assert.False(t, popped)

}

// requirePop is a helper requiring that pop does not error
func requirePop(t *testing.T, q *dispatcher.TargetQueue) dispatcher.Target {
	req, popped := q.Pop()
	require.True(t, popped)
	return req
}

// chainInfoFromHeight is a helper that constructs a unique chain info off of
// an int. The tipset key is a faked cid from the string of that integer and
// the height is that integer.
func chainInfoFromHeight(t *testing.T, h int) *block.ChainInfo {
	hStr := strconv.Itoa(h)
	c := types.CidFromString(t, hStr)
	return &block.ChainInfo{
		Head:   block.NewTipSetKey(c),
		Height: abi.ChainEpoch(h),
	}
}
