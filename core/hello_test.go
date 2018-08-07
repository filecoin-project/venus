package core

import (
	"context"
	"testing"
	"time"

	mocknet "gx/ipfs/QmY51bqSM5XgxQZqsBrQcRkKTnCb8EKpJpR9K6Qax7Njco/go-libp2p/p2p/net/mock"
	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	peer "gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	types "github.com/filecoin-project/go-filecoin/types"
)

type mockSyncCallback struct {
	mock.Mock
}

func (msb *mockSyncCallback) SyncCallback(p peer.ID, cids []*cid.Cid, h uint64) {
	msb.Called(p, cids, h)
}

type mockHeaviestGetter struct {
	heaviest TipSet
}

func (mhg *mockHeaviestGetter) getHeaviestTipSet() TipSet {
	return mhg.heaviest
}

func TestHelloHandshake(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require := require.New(t)

	mn, err := mocknet.WithNPeers(ctx, 2)
	assert.NoError(t, err)

	a := mn.Hosts()[0]
	b := mn.Hosts()[1]

	genesisA := &types.Block{Nonce: 451}

	heavy1 := RequireNewTipSet(require, &types.Block{Nonce: 1000, Height: 2})
	heavy2 := RequireNewTipSet(require, &types.Block{Nonce: 1001, Height: 3})

	msc1, msc2 := new(mockSyncCallback), new(mockSyncCallback)
	hg1, hg2 := &mockHeaviestGetter{heavy1}, &mockHeaviestGetter{heavy2}

	h1 := NewHello(a, genesisA.Cid(), msc1.SyncCallback, hg1.getHeaviestTipSet)
	h2 := NewHello(b, genesisA.Cid(), msc2.SyncCallback, hg2.getHeaviestTipSet)
	_, _ = h1, h2

	msc1.On("SyncCallback", b.ID(), heavy2.ToSortedCidSet().ToSlice(), uint64(3)).Return()
	msc2.On("SyncCallback", a.ID(), heavy1.ToSortedCidSet().ToSlice(), uint64(2)).Return()

	mn.LinkAll()
	mn.ConnectAllButSelf()

	time.Sleep(time.Millisecond * 50)

	msc1.AssertExpectations(t)
	msc2.AssertExpectations(t)
}

func TestHelloBadGenesis(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require := require.New(t)

	mn, err := mocknet.WithNPeers(ctx, 2)
	assert.NoError(t, err)

	a := mn.Hosts()[0]
	b := mn.Hosts()[1]

	genesisA := &types.Block{Nonce: 451}
	genesisB := &types.Block{Nonce: 101}

	heavy1 := RequireNewTipSet(require, &types.Block{Nonce: 1000, Height: 2})
	heavy2 := RequireNewTipSet(require, &types.Block{Nonce: 1001, Height: 3})

	msc1, msc2 := new(mockSyncCallback), new(mockSyncCallback)
	hg1, hg2 := &mockHeaviestGetter{heavy1}, &mockHeaviestGetter{heavy2}

	h1 := NewHello(a, genesisA.Cid(), msc1.SyncCallback, hg1.getHeaviestTipSet)
	h2 := NewHello(b, genesisB.Cid(), msc2.SyncCallback, hg2.getHeaviestTipSet)
	_, _ = h1, h2

	msc1.On("SyncCallback", mock.Anything, mock.Anything, mock.Anything).Return()
	msc2.On("SyncCallback", mock.Anything, mock.Anything, mock.Anything).Return()

	mn.LinkAll()
	mn.ConnectAllButSelf()

	time.Sleep(time.Millisecond * 50)

	msc1.AssertNumberOfCalls(t, "SyncCallback", 0)
	msc2.AssertNumberOfCalls(t, "SyncCallback", 0)
}

func TestHelloMultiBlock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require := require.New(t)

	mn, err := mocknet.WithNPeers(ctx, 2)
	assert.NoError(t, err)

	a := mn.Hosts()[0]
	b := mn.Hosts()[1]

	genesisA := &types.Block{Nonce: 452}

	heavy1 := RequireNewTipSet(require,
		&types.Block{Nonce: 1000, Height: 2},
		&types.Block{Nonce: 1002, Height: 2},
		&types.Block{Nonce: 1004, Height: 2},
	)
	heavy2 := RequireNewTipSet(require,
		&types.Block{Nonce: 1001, Height: 3},
		&types.Block{Nonce: 1003, Height: 3},
		&types.Block{Nonce: 1005, Height: 3},
	)

	msc1, msc2 := new(mockSyncCallback), new(mockSyncCallback)
	hg1, hg2 := &mockHeaviestGetter{heavy1}, &mockHeaviestGetter{heavy2}

	h1 := NewHello(a, genesisA.Cid(), msc1.SyncCallback, hg1.getHeaviestTipSet)
	h2 := NewHello(b, genesisA.Cid(), msc2.SyncCallback, hg2.getHeaviestTipSet)
	_, _ = h1, h2

	msc1.On("SyncCallback", b.ID(), heavy2.ToSortedCidSet().ToSlice(), uint64(3)).Return()
	msc2.On("SyncCallback", a.ID(), heavy1.ToSortedCidSet().ToSlice(), uint64(2)).Return()

	mn.LinkAll()
	mn.ConnectAllButSelf()

	time.Sleep(time.Millisecond * 50)

	msc1.AssertExpectations(t)
	msc2.AssertExpectations(t)
}
