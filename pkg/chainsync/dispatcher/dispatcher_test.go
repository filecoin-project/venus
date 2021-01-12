package dispatcher_test

import (
	"context"
	fbig "github.com/filecoin-project/go-state-types/big"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/venus/pkg/specactors/builtin"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/chainsync/dispatcher"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/util/moresync"
)

type mockSyncer struct {
	headsCalled []*block.TipSet
}

func (fs *mockSyncer) Staged() *block.TipSet {
	panic("implement me")
}

func (fs *mockSyncer) HandleNewTipSet(_ context.Context, ci *block.ChainInfo) error {
	fs.headsCalled = append(fs.headsCalled, ci.Head)
	return nil
}

func TestDispatchStartHappy(t *testing.T) {
	tf.UnitTest(t)
	s := &mockSyncer{
		headsCalled: make([]*block.TipSet, 0),
	}
	testDispatch := dispatcher.NewDispatcher(s)

	cis := []*block.ChainInfo{
		// We need to put these in priority order to avoid a race.
		// If we send 0 before 42, it is possible the dispatcher will
		// pick up 0 and start processing before it sees 42.
		chainInfoWithHeightAndWeight(t, 42, 1),
		chainInfoWithHeightAndWeight(t, 16, 2),
		chainInfoWithHeightAndWeight(t, 3, 3),
		chainInfoWithHeightAndWeight(t, 2, 4),
		chainInfoWithHeightAndWeight(t, 0, 5),
	}

	testDispatch.Start(context.Background())

	// set up a blocking channel and register to unblock after 5 synced
	allDone := moresync.NewLatch(5)
	testDispatch.RegisterCallback(func(t *dispatcher.Target, _ error) {
		allDone.Done()
	})

	// receive requests before Start() to test deterministic order
	go func() {
		for _, ci := range cis {
			assert.NoError(t, testDispatch.SendHello(ci))
		}
	}()
	allDone.Wait()
	sort.Slice(cis, func(i, j int) bool {
		weigtI, _ := cis[i].Head.ParentWeight()
		weigtJ, _ := cis[j].Head.ParentWeight()
		return weigtI.GreaterThan(weigtJ)
	})
	// check that the mockSyncer synced in order
	require.Equal(t, 5, len(s.headsCalled))
	for i := range cis {
		assert.Equal(t, cis[i].Head.EnsureHeight(), s.headsCalled[i].EnsureHeight())
	}
}

func TestDispatcherDropsWhenFull(t *testing.T) {
	tf.UnitTest(t)
	s := &mockSyncer{
		headsCalled: make([]*block.TipSet, 0),
	}
	testWorkSize := 20
	testBufferSize := 30
	testDispatch := dispatcher.NewDispatcherWithSizes(s, testWorkSize, testBufferSize)

	finished := moresync.NewLatch(1)
	testDispatch.RegisterCallback(func(target *dispatcher.Target, _ error) {
		// Fail if the work that should be dropped gets processed
		assert.False(t, target.Head.EnsureHeight() == 100)
		assert.False(t, target.Head.EnsureHeight() == 101)
		assert.False(t, target.Head.EnsureHeight() == 102)
		if target.Head.EnsureHeight() == 0 {
			// 0 has lowest priority of non-dropped
			finished.Done()
		}
	})
	for j := 0; j < testWorkSize; j++ {
		ci := chainInfoWithHeightAndWeight(t, j, 1001)
		assert.NoError(t, testDispatch.SendHello(ci))
	}
	// Should be dropped
	assert.NoError(t, testDispatch.SendHello(chainInfoWithHeightAndWeight(t, 100, 1001)))
	assert.NoError(t, testDispatch.SendHello(chainInfoWithHeightAndWeight(t, 101, 1001)))
	assert.NoError(t, testDispatch.SendHello(chainInfoWithHeightAndWeight(t, 102, 1001)))

	testDispatch.Start(context.Background())

	finished.Wait()
}

func TestQueueHappy(t *testing.T) {
	tf.UnitTest(t)
	testQ := dispatcher.NewTargetTracker(20)

	// Add syncRequests out of order
	sR0 := &dispatcher.Target{ChainInfo: *(chainInfoWithHeightAndWeight(t, 0, 1001))}
	sR1 := &dispatcher.Target{ChainInfo: *(chainInfoWithHeightAndWeight(t, 1, 1001))}
	sR2 := &dispatcher.Target{ChainInfo: *(chainInfoWithHeightAndWeight(t, 2, 1001))}
	sR47 := &dispatcher.Target{ChainInfo: *(chainInfoWithHeightAndWeight(t, 47, 1001))}

	testQ.Add(sR2)
	testQ.Add(sR47)
	testQ.Add(sR0)
	testQ.Add(sR1)

	assert.Equal(t, 4, testQ.Len())

	// Pop in order
	out0 := requirePop(t, testQ)

	weight, _ := out0.ChainInfo.Head.ParentWeight()
	assert.Equal(t, int64(1001), weight.Int.Int64())
}

func TestQueueDuplicates(t *testing.T) {
	tf.UnitTest(t)
	testQ := dispatcher.NewTargetTracker(20)

	// Add syncRequests with same height
	sR0 := &dispatcher.Target{ChainInfo: *(chainInfoWithHeightAndWeight(t, 0, 1001))}
	sR0dup := &dispatcher.Target{ChainInfo: *(chainInfoWithHeightAndWeight(t, 0, 1001))}

	testQ.Add(sR0)
	testQ.Add(sR0dup)

	// Only one of these makes it onto the queue
	assert.Equal(t, 1, testQ.Len())

	// Pop
	first := requirePop(t, testQ)
	assert.Equal(t, abi.ChainEpoch(0), first.ChainInfo.Head.EnsureHeight())
	testQ.Remove(first)
	// Now if we push the duplicate it goes back on
	testQ.Add(sR0dup)
	assert.Equal(t, 1, testQ.Len())

	second := requirePop(t, testQ)
	assert.Equal(t, abi.ChainEpoch(0), second.ChainInfo.Head.EnsureHeight())
}

func TestQueueEmptyPopErrors(t *testing.T) {
	tf.UnitTest(t)
	testQ := dispatcher.NewTargetTracker(20)
	sR0 := &dispatcher.Target{ChainInfo: *(chainInfoWithHeightAndWeight(t, 0, 1002))}
	sR47 := &dispatcher.Target{ChainInfo: *(chainInfoWithHeightAndWeight(t, 47, 1001))}

	// Add 2
	testQ.Add(sR47)
	testQ.Add(sR0)

	// Pop 3
	assert.Equal(t, 2, testQ.Len())
	first := requirePop(t, testQ)
	testQ.Remove(first)
	assert.Equal(t, 1, testQ.Len())

	second := requirePop(t, testQ)
	testQ.Remove(second)
	assert.Equal(t, 0, testQ.Len())

	_, popped := testQ.Select()
	assert.False(t, popped)

}

// requirePop is a helper requiring that pop does not error
func requirePop(t *testing.T, q *dispatcher.TargetTracker) *dispatcher.Target {
	req, popped := q.Select()
	require.True(t, popped)
	return req
}

// chainInfoWithHeightAndWeight is a helper that constructs a unique chain info off of
// an int. The tipset key is a faked cid from the string of that integer and
// the height is that integer.
func chainInfoWithHeightAndWeight(t *testing.T, h int, weight int64) *block.ChainInfo {
	newAddress := types.NewForTestGetter()
	posts := []builtin.PoStProof{{PoStProof: abi.RegisteredPoStProof_StackedDrgWinning32GiBV1, ProofBytes: []byte{0x07}}}
	blk := &block.Block{
		Miner:         newAddress(),
		Ticket:        block.Ticket{VRFProof: []byte{0x03, 0x01, 0x02}},
		ElectionProof: &block.ElectionProof{VRFProof: []byte{0x0c, 0x0d}},
		BeaconEntries: []*block.BeaconEntry{
			{
				Round: 44,
				Data:  []byte{0xc0},
			},
		},
		Height:                abi.ChainEpoch(h),
		Messages:              types.CidFromString(t, "someothercid"),
		ParentMessageReceipts: types.CidFromString(t, "someothercid"),
		Parents:               block.NewTipSetKey(types.CidFromString(t, "someothercid")),
		ParentWeight:          fbig.NewInt(weight),
		ForkSignaling:         2,
		ParentStateRoot:       types.CidFromString(t, "someothercid"),
		Timestamp:             4,
		ParentBaseFee:         abi.NewTokenAmount(20),
		WinPoStProof:          block.FromAbiProofArr(posts),
		BlockSig: &acrypto.Signature{
			Type: acrypto.SigTypeBLS,
			Data: []byte{0x4},
		},
	}
	b, _ := block.NewTipSet(blk)
	return &block.ChainInfo{
		Head: b,
	}
}
