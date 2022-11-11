// stm: #unit
package dispatcher_test

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/pkg/testhelpers"

	fbig "github.com/filecoin-project/go-state-types/big"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	proof2 "github.com/filecoin-project/specs-actors/v2/actors/runtime/proof"
	syncTypes "github.com/filecoin-project/venus/pkg/chainsync/types"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/chainsync/dispatcher"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

type mockSyncer struct {
	headsCalled []*types.TipSet
}

func (fs *mockSyncer) Head() *types.TipSet {
	return types.UndefTipSet
}

func (fs *mockSyncer) HandleNewTipSet(_ context.Context, ci *syncTypes.Target) error {
	fs.headsCalled = append(fs.headsCalled, ci.Head)
	return nil
}

func TestDispatchStartHappy(t *testing.T) {
	tf.UnitTest(t)
	s := &mockSyncer{
		headsCalled: make([]*types.TipSet, 0),
	}
	testDispatch := dispatcher.NewDispatcher(s)

	cis := []*types.ChainInfo{
		// We need to put these in priority order to avoid a race.
		// If we send 0 before 42, it is possible the dispatcher will
		// pick up 0 and start processing before it sees 42.
		chainInfoWithHeightAndWeight(t, 42, 1),
		chainInfoWithHeightAndWeight(t, 16, 2),
		chainInfoWithHeightAndWeight(t, 3, 3),
		chainInfoWithHeightAndWeight(t, 2, 4),
		chainInfoWithHeightAndWeight(t, 1, 5),
	}
	// stm: @CHAINSYNC_DISPATCHER_SET_CONCURRENT_001
	testDispatch.SetConcurrent(2)

	// stm: @CHAINSYNC_DISPATCHER_CONCURRENT_001
	assert.Equal(t, testDispatch.Concurrent(), int64(2))

	// stm: @CHAINSYNC_DISPATCHER_START_001
	testDispatch.Start(context.Background())

	t.Logf("waiting for 'syncWorker' input channel standby for 100(ms)")
	time.Sleep(time.Millisecond * 100)

	// set up a blocking channel and register to unblock after 5 synced
	waitCh := make(chan struct{})
	// stm: @CHAINSYNC_DISPATCHER_REGISTER_CALLBACK_001
	testDispatch.RegisterCallback(func(target *syncTypes.Target, _ error) {
		if target.Head.Key().Equals(cis[4].Head.Key()) {
			waitCh <- struct{}{}
		}
	})

	// receive requests before Start() to test deterministic order
	for _, ci := range cis {
		go func(info *types.ChainInfo) {
			assert.NoError(t, testDispatch.SendHello(info))
		}(ci)
	}

	select {
	case <-waitCh:
	case <-time.After(time.Second * 5):
		assert.Failf(t, "", "couldn't waited a correct chain syncing target in 5(s)")
	}
}

func TestQueueHappy(t *testing.T) {
	tf.UnitTest(t)
	testQ := syncTypes.NewTargetTracker(20)

	// Add syncRequests out of order
	sR0 := &syncTypes.Target{ChainInfo: *(chainInfoWithHeightAndWeight(t, 0, 1001))}
	sR1 := &syncTypes.Target{ChainInfo: *(chainInfoWithHeightAndWeight(t, 1, 1001))}
	sR2 := &syncTypes.Target{ChainInfo: *(chainInfoWithHeightAndWeight(t, 2, 1001))}
	sR47 := &syncTypes.Target{ChainInfo: *(chainInfoWithHeightAndWeight(t, 47, 1001))}

	testQ.Add(sR2)
	testQ.Add(sR47)
	testQ.Add(sR0)
	testQ.Add(sR1)

	assert.Equal(t, 1, testQ.Len())

	// Pop in order
	out0 := requirePop(t, testQ)

	weight := out0.ChainInfo.Head.ParentWeight()
	assert.Equal(t, int64(1001), weight.Int.Int64())
}

func TestQueueDuplicates(t *testing.T) {
	tf.UnitTest(t)
	testQ := syncTypes.NewTargetTracker(20)

	// Add syncRequests with same height
	sR0 := &syncTypes.Target{ChainInfo: *(chainInfoWithHeightAndWeight(t, 0, 1001))}
	sR0dup := &syncTypes.Target{ChainInfo: *(chainInfoWithHeightAndWeight(t, 0, 1001))}

	testQ.Add(sR0)
	testQ.Add(sR0dup)

	// Only one of these makes it onto the queue
	assert.Equal(t, 1, testQ.Len())

	// Pop
	first := requirePop(t, testQ)
	assert.Equal(t, abi.ChainEpoch(0), first.ChainInfo.Head.Height())
	testQ.Remove(first)
}

func TestQueueEmptyPopErrors(t *testing.T) {
	tf.UnitTest(t)
	testQ := syncTypes.NewTargetTracker(20)
	sR0 := &syncTypes.Target{ChainInfo: *(chainInfoWithHeightAndWeight(t, 0, 1002))}
	sR47 := &syncTypes.Target{ChainInfo: *(chainInfoWithHeightAndWeight(t, 47, 1001))}

	// Add 2
	testQ.Add(sR47)
	testQ.Add(sR0)

	// Pop 3
	assert.Equal(t, 1, testQ.Len())
	first := requirePop(t, testQ)
	testQ.Remove(first)
	assert.Equal(t, 0, testQ.Len())
}

// requirePop is a helper requiring that pop does not error
func requirePop(t *testing.T, q *syncTypes.TargetTracker) *syncTypes.Target {
	req, popped := q.Select()
	require.True(t, popped)
	return req
}

// chainInfoWithHeightAndWeight is a helper that constructs a unique chain info off of
// an int. The tipset key is a faked cid from the string of that integer and
// the height is that integer.
func chainInfoWithHeightAndWeight(t *testing.T, h int, weight int64) *types.ChainInfo {
	newAddress := testhelpers.NewForTestGetter()
	posts := []proof2.PoStProof{{PoStProof: abi.RegisteredPoStProof_StackedDrgWinning32GiBV1, ProofBytes: []byte{0x07}}}
	blk := &types.BlockHeader{
		Miner:         newAddress(),
		Ticket:        &types.Ticket{VRFProof: []byte{0x03, 0x01, 0x02}},
		ElectionProof: &types.ElectionProof{VRFProof: []byte{0x0c, 0x0d}},
		BeaconEntries: []types.BeaconEntry{
			{
				Round: 44,
				Data:  []byte{0xc0},
			},
		},
		Height:                abi.ChainEpoch(h),
		Messages:              testhelpers.CidFromString(t, "someothercid"),
		ParentMessageReceipts: testhelpers.CidFromString(t, "someothercid"),
		Parents:               []cid.Cid{testhelpers.CidFromString(t, "someothercid")},
		ParentWeight:          fbig.NewInt(weight),
		ForkSignaling:         2,
		ParentStateRoot:       testhelpers.CidFromString(t, "someothercid"),
		Timestamp:             4,
		ParentBaseFee:         abi.NewTokenAmount(20),
		WinPoStProof:          posts,
		BlockSig: &acrypto.Signature{
			Type: acrypto.SigTypeBLS,
			Data: []byte{0x4},
		},
	}
	b, _ := types.NewTipSet([]*types.BlockHeader{blk})
	return &types.ChainInfo{
		Head: b,
	}
}
