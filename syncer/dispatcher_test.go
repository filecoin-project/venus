package syncer_test

import(
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"	

	"github.com/filecoin-project/go-filecoin/syncer"
	"github.com/filecoin-project/go-filecoin/types"	
)


func TestNewDispatcher(t *testing.T) {
	syncer.NewDispatcher()
}

func TestQueueHappy(t *testing.T) {
	testQ := syncer.NewTargetQueue()

	// Add syncRequests out of order
	sR0 := &syncer.SyncRequest{ChainInfo: types.ChainInfo{Height: 0}}
	sR1 := &syncer.SyncRequest{ChainInfo: types.ChainInfo{Height: 1}}
	sR2 := &syncer.SyncRequest{ChainInfo: types.ChainInfo{Height: 2}}
	sR47 := &syncer.SyncRequest{ChainInfo: types.ChainInfo{Height: 47}}

	requirePush(t, sR2, testQ)
	requirePush(t, sR47, testQ)
	requirePush(t, sR0, testQ)
	requirePush(t, sR1, testQ)

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

// requirePop is a helper requiring that pop does not error
func requirePop(t *testing.T, q *syncer.TargetQueue) *syncer.SyncRequest {
	req, err := q.Pop()
	require.NoError(t, err)
	return req
}

// requirePush is a helper requiring that push does not error
func requirePush(t *testing.T, req *syncer.SyncRequest, q *syncer.TargetQueue) {
	require.NoError(t, q.Push(req))
}


func TestQueueDuplicates(t *testing.T) {
	testQ := syncer.NewTargetQueue()

	// Add syncRequests with same height
	sR0 := &syncer.SyncRequest{ChainInfo: types.ChainInfo{Height: 0}}
	sR0dup := &syncer.SyncRequest{ChainInfo: types.ChainInfo{Height: 0}}

	err := testQ.Push(sR0)
	assert.NoError(t, err)

	err = testQ.Push(sR0dup)
	assert.NoError(t, err)	
	
	// Pop twice
	first := requirePop(t, testQ)
	second := requirePop(t, testQ)	
	
	assert.Equal(t, uint64(0), first.ChainInfo.Height)
	assert.Equal(t, uint64(0), second.ChainInfo.Height)	
}

func TestQueueEmptyPop(t *testing.T) {
	testQ := syncer.NewTargetQueue()
	sR0 := &syncer.SyncRequest{ChainInfo: types.ChainInfo{Height: 0}}
	sR47 := &syncer.SyncRequest{ChainInfo: types.ChainInfo{Height: 47}}

	// Push 2
	requirePush(t, sR47, testQ)
	requirePush(t, sR0, testQ)

	// Pop 3
	assert.Equal(t, 2, testQ.Len())
	_ = requirePop(t, testQ)
	assert.Equal(t, 1, testQ.Len())
	_ = requirePop(t, testQ)
	assert.Equal(t, 0, testQ.Len())

	_, err := testQ.Pop()
	assert.Error(t, err)
	assert.Equal(t, 0, testQ.Len())	
}
