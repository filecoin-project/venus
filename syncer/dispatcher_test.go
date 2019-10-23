package syncer_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/filecoin-project/go-filecoin/block"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/syncer"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/util/moresync"
)

type mockSyncer struct {
	headsCalled []block.TipSetKey
}

func (fs *mockSyncer) HandleNewTipSet(ctx context.Context, ci *block.ChainInfo, t bool) error {
	fs.headsCalled = append(fs.headsCalled, ci.Head)
	return nil
}

func TestDispatchStartHappy(t *testing.T) {
	tf.UnitTest(t)
	s := &mockSyncer{
		headsCalled: make([]block.TipSetKey, 0),
	}
	testDispatch := syncer.NewDispatcher(s)

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
	testDispatch.RegisterCallback(func(t syncer.Target) { allDone.Done() })

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

func TestQueueHappy(t *testing.T) {
	tf.UnitTest(t)
	testQ := syncer.NewTargetQueue()

	// Add syncRequests out of order
	sR0 := syncer.Target{ChainInfo: *(chainInfoFromHeight(t, 0))}
	sR1 := syncer.Target{ChainInfo: *(chainInfoFromHeight(t, 1))}
	sR2 := syncer.Target{ChainInfo: *(chainInfoFromHeight(t, 2))}
	sR47 := syncer.Target{ChainInfo: *(chainInfoFromHeight(t, 47))}

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

	assert.Equal(t, uint64(47), out0.ChainInfo.Height)
	assert.Equal(t, uint64(2), out1.ChainInfo.Height)
	assert.Equal(t, uint64(1), out2.ChainInfo.Height)
	assert.Equal(t, uint64(0), out3.ChainInfo.Height)

	assert.Equal(t, 0, testQ.Len())
}

func TestQueueDuplicates(t *testing.T) {
	tf.UnitTest(t)
	testQ := syncer.NewTargetQueue()

	// Add syncRequests with same height
	sR0 := syncer.Target{ChainInfo: *(chainInfoFromHeight(t, 0))}
	sR0dup := syncer.Target{ChainInfo: *(chainInfoFromHeight(t, 0))}

	testQ.Push(sR0)
	testQ.Push(sR0dup)

	// Only one of these makes it onto the queue
	assert.Equal(t, 1, testQ.Len())

	// Pop
	first := requirePop(t, testQ)
	assert.Equal(t, uint64(0), first.ChainInfo.Height)

	// Now if we push the duplicate it goes back on
	testQ.Push(sR0dup)
	assert.Equal(t, 1, testQ.Len())

	second := requirePop(t, testQ)
	assert.Equal(t, uint64(0), second.ChainInfo.Height)
}

func TestQueueEmptyPopErrors(t *testing.T) {
	tf.UnitTest(t)
	testQ := syncer.NewTargetQueue()
	sR0 := syncer.Target{ChainInfo: *(chainInfoFromHeight(t, 0))}
	sR47 := syncer.Target{ChainInfo: *(chainInfoFromHeight(t, 47))}

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
func requirePop(t *testing.T, q *syncer.TargetQueue) syncer.Target {
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
		Height: uint64(h),
	}
}
