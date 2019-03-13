package hello

import (
	"context"
	"testing"
	"time"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"
	"gx/ipfs/QmcNGX5RaxPPCYwa6yGXM1EcUbrreTTinixLcYGmMwf1sx/go-libp2p/p2p/net/mock"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/mock"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
)

type mockSyncCallback struct {
	mock.Mock
}

func (msb *mockSyncCallback) SyncCallback(p peer.ID, cids []cid.Cid, h uint64) {
	msb.Called(p, cids, h)
}

type mockHeaviestGetter struct {
	heaviest types.TipSet
}

func (mhg *mockHeaviestGetter) getHeaviestTipSet() types.TipSet {
	return mhg.heaviest
}

func TestHelloHandshake(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mn, err := mocknet.WithNPeers(ctx, 2)
	require.NoError(err)

	a := mn.Hosts()[0]
	b := mn.Hosts()[1]

	genesisA := &types.Block{Nonce: 451}

	heavy1 := th.RequireNewTipSet(require, &types.Block{Nonce: 1000, Height: 2})
	heavy2 := th.RequireNewTipSet(require, &types.Block{Nonce: 1001, Height: 3})

	msc1, msc2 := new(mockSyncCallback), new(mockSyncCallback)
	hg1, hg2 := &mockHeaviestGetter{heavy1}, &mockHeaviestGetter{heavy2}

	New(a, genesisA.Cid(), msc1.SyncCallback, hg1.getHeaviestTipSet, "", "")
	New(b, genesisA.Cid(), msc2.SyncCallback, hg2.getHeaviestTipSet, "", "")

	msc1.On("SyncCallback", b.ID(), heavy2.ToSortedCidSet().ToSlice(), uint64(3)).Return()
	msc2.On("SyncCallback", a.ID(), heavy1.ToSortedCidSet().ToSlice(), uint64(2)).Return()

	require.NoError(mn.LinkAll())
	require.NoError(mn.ConnectAllButSelf())

	require.NoError(th.WaitForIt(10, 50*time.Millisecond, func() (bool, error) {
		var msc1Done bool
		var msc2Done bool
		for _, call := range msc1.Calls {
			if call.Method == "SyncCallback" {
				if _, differences := msc1.ExpectedCalls[0].Arguments.Diff(call.Arguments); differences == 0 {
					msc1Done = true
					break
				}
			}
		}
		for _, call := range msc2.Calls {
			if call.Method == "SyncCallback" {
				if _, differences := msc2.ExpectedCalls[0].Arguments.Diff(call.Arguments); differences == 0 {
					msc2Done = true
					break
				}
			}
		}

		return msc1Done && msc2Done, nil
	}))
}

func TestHelloBadGenesis(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require := require.New(t)

	mn, err := mocknet.WithNPeers(ctx, 2)
	assert.NoError(t, err)

	a := mn.Hosts()[0]
	b := mn.Hosts()[1]

	genesisA := &types.Block{Nonce: 451}
	genesisB := &types.Block{Nonce: 101}

	heavy1 := th.RequireNewTipSet(require, &types.Block{Nonce: 1000, Height: 2})
	heavy2 := th.RequireNewTipSet(require, &types.Block{Nonce: 1001, Height: 3})

	msc1, msc2 := new(mockSyncCallback), new(mockSyncCallback)
	hg1, hg2 := &mockHeaviestGetter{heavy1}, &mockHeaviestGetter{heavy2}

	New(a, genesisA.Cid(), msc1.SyncCallback, hg1.getHeaviestTipSet, "", "")
	New(b, genesisB.Cid(), msc2.SyncCallback, hg2.getHeaviestTipSet, "", "")

	msc1.On("SyncCallback", mock.Anything, mock.Anything, mock.Anything).Return()
	msc2.On("SyncCallback", mock.Anything, mock.Anything, mock.Anything).Return()

	require.NoError(mn.LinkAll())
	require.NoError(mn.ConnectAllButSelf())

	time.Sleep(time.Millisecond * 50)

	msc1.AssertNumberOfCalls(t, "SyncCallback", 0)
	msc2.AssertNumberOfCalls(t, "SyncCallback", 0)
}

func TestHelloWrongVersion(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require := require.New(t)

	mn, err := mocknet.WithNPeers(ctx, 2)
	assert.NoError(t, err)

	a, b := mn.Hosts()[0], mn.Hosts()[1]

	genesisA := &types.Block{Nonce: 451}

	heavy := th.RequireNewTipSet(require, &types.Block{Nonce: 1000, Height: 2})

	msc1, msc2 := new(mockSyncCallback), new(mockSyncCallback)
	hg := &mockHeaviestGetter{heavy}

	New(a, genesisA.Cid(), msc1.SyncCallback, hg.getHeaviestTipSet, "devnet-user", "sha1")
	msc1.On("SyncCallback", mock.Anything, mock.Anything, mock.Anything).Return()

	New(b, genesisA.Cid(), msc2.SyncCallback, hg.getHeaviestTipSet, "devnet-user", "sha2")
	msc2.On("SyncCallback", mock.Anything, mock.Anything, mock.Anything).Return()

	require.NoError(mn.LinkAll())
	require.NoError(mn.ConnectAllButSelf())

	time.Sleep(time.Millisecond * 50)

	msc1.AssertNumberOfCalls(t, "SyncCallback", 0)
	msc2.AssertNumberOfCalls(t, "SyncCallback", 0)
}

func TestHelloMultiBlock(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require := require.New(t)

	mn, err := mocknet.WithNPeers(ctx, 2)
	assert.NoError(t, err)

	a := mn.Hosts()[0]
	b := mn.Hosts()[1]

	genesisA := &types.Block{Nonce: 452}

	heavy1 := th.RequireNewTipSet(require,
		&types.Block{Nonce: 1000, Height: 2},
		&types.Block{Nonce: 1002, Height: 2},
		&types.Block{Nonce: 1004, Height: 2},
	)
	heavy2 := th.RequireNewTipSet(require,
		&types.Block{Nonce: 1001, Height: 3},
		&types.Block{Nonce: 1003, Height: 3},
		&types.Block{Nonce: 1005, Height: 3},
	)

	msc1, msc2 := new(mockSyncCallback), new(mockSyncCallback)
	hg1, hg2 := &mockHeaviestGetter{heavy1}, &mockHeaviestGetter{heavy2}

	New(a, genesisA.Cid(), msc1.SyncCallback, hg1.getHeaviestTipSet, "", "")
	New(b, genesisA.Cid(), msc2.SyncCallback, hg2.getHeaviestTipSet, "", "")

	msc1.On("SyncCallback", b.ID(), heavy2.ToSortedCidSet().ToSlice(), uint64(3)).Return()
	msc2.On("SyncCallback", a.ID(), heavy1.ToSortedCidSet().ToSlice(), uint64(2)).Return()

	mn.LinkAll()
	mn.ConnectAllButSelf()

	time.Sleep(time.Millisecond * 50)

	msc1.AssertExpectations(t)
	msc2.AssertExpectations(t)
}
