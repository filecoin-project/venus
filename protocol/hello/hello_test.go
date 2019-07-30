package hello

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/p2p/net/mock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

type mockHelloCallback struct {
	mock.Mock
}

func (msb *mockHelloCallback) HelloCallback(ci *types.ChainInfo) {
	msb.Called(ci.Peer, ci.Head, ci.Height)
}

type mockHeaviestGetter struct {
	heaviest types.TipSet
}

func (mhg *mockHeaviestGetter) getHeaviestTipSet() (types.TipSet, error) {
	return mhg.heaviest, nil
}

func TestHelloHandshake(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mn, err := mocknet.WithNPeers(ctx, 2)
	require.NoError(t, err)

	a := mn.Hosts()[0]
	b := mn.Hosts()[1]

	genesisA := &types.Block{Nonce: 451}

	heavy1 := th.RequireNewTipSet(t, &types.Block{Nonce: 1000, Height: 2})
	heavy2 := th.RequireNewTipSet(t, &types.Block{Nonce: 1001, Height: 3})

	msc1, msc2 := new(mockHelloCallback), new(mockHelloCallback)
	hg1, hg2 := &mockHeaviestGetter{heavy1}, &mockHeaviestGetter{heavy2}

	New(a, genesisA.Cid(), msc1.HelloCallback, hg1.getHeaviestTipSet, "", "")
	New(b, genesisA.Cid(), msc2.HelloCallback, hg2.getHeaviestTipSet, "", "")

	msc1.On("HelloCallback", b.ID(), heavy2.Key(), uint64(3)).Return()
	msc2.On("HelloCallback", a.ID(), heavy1.Key(), uint64(2)).Return()

	require.NoError(t, mn.LinkAll())
	require.NoError(t, mn.ConnectAllButSelf())

	require.NoError(t, th.WaitForIt(10, 50*time.Millisecond, func() (bool, error) {
		var msc1Done bool
		var msc2Done bool
		for _, call := range msc1.Calls {
			if call.Method == "HelloCallback" {
				if _, differences := msc1.ExpectedCalls[0].Arguments.Diff(call.Arguments); differences == 0 {
					msc1Done = true
					break
				}
			}
		}
		for _, call := range msc2.Calls {
			if call.Method == "HelloCallback" {
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
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mn, err := mocknet.WithNPeers(ctx, 2)
	assert.NoError(t, err)

	a := mn.Hosts()[0]
	b := mn.Hosts()[1]

	genesisA := &types.Block{Nonce: 451}
	genesisB := &types.Block{Nonce: 101}

	heavy1 := th.RequireNewTipSet(t, &types.Block{Nonce: 1000, Height: 2})
	heavy2 := th.RequireNewTipSet(t, &types.Block{Nonce: 1001, Height: 3})

	msc1, msc2 := new(mockHelloCallback), new(mockHelloCallback)
	hg1, hg2 := &mockHeaviestGetter{heavy1}, &mockHeaviestGetter{heavy2}

	New(a, genesisA.Cid(), msc1.HelloCallback, hg1.getHeaviestTipSet, "", "")
	New(b, genesisB.Cid(), msc2.HelloCallback, hg2.getHeaviestTipSet, "", "")

	msc1.On("HelloCallback", mock.Anything, mock.Anything, mock.Anything).Return()
	msc2.On("HelloCallback", mock.Anything, mock.Anything, mock.Anything).Return()

	require.NoError(t, mn.LinkAll())
	require.NoError(t, mn.ConnectAllButSelf())

	time.Sleep(time.Millisecond * 50)

	msc1.AssertNumberOfCalls(t, "HelloCallback", 0)
	msc2.AssertNumberOfCalls(t, "HelloCallback", 0)
}

func TestHelloWrongVersion(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mn, err := mocknet.WithNPeers(ctx, 2)
	assert.NoError(t, err)

	a, b := mn.Hosts()[0], mn.Hosts()[1]

	genesisA := &types.Block{Nonce: 451}

	heavy := th.RequireNewTipSet(t, &types.Block{Nonce: 1000, Height: 2})

	msc1, msc2 := new(mockHelloCallback), new(mockHelloCallback)
	hg := &mockHeaviestGetter{heavy}

	New(a, genesisA.Cid(), msc1.HelloCallback, hg.getHeaviestTipSet, "devnet-user", "sha1")
	msc1.On("HelloCallback", mock.Anything, mock.Anything, mock.Anything).Return()

	New(b, genesisA.Cid(), msc2.HelloCallback, hg.getHeaviestTipSet, "devnet-user", "sha2")
	msc2.On("HelloCallback", mock.Anything, mock.Anything, mock.Anything).Return()

	require.NoError(t, mn.LinkAll())
	require.NoError(t, mn.ConnectAllButSelf())

	time.Sleep(time.Millisecond * 50)

	msc1.AssertNumberOfCalls(t, "HelloCallback", 0)
	msc2.AssertNumberOfCalls(t, "HelloCallback", 0)
}

func TestHelloWrongVersionTestDevnet(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mn, err := mocknet.WithNPeers(ctx, 2)
	assert.NoError(t, err)

	a, b := mn.Hosts()[0], mn.Hosts()[1]

	genesisA := &types.Block{Nonce: 451}

	heavy := th.RequireNewTipSet(t, &types.Block{Nonce: 1000, Height: 2})

	msc1, msc2 := new(mockHelloCallback), new(mockHelloCallback)
	hg := &mockHeaviestGetter{heavy}

	New(a, genesisA.Cid(), msc1.HelloCallback, hg.getHeaviestTipSet, "devnet-staging", "sha1")
	msc1.On("HelloCallback", mock.Anything, mock.Anything, mock.Anything).Return()

	New(b, genesisA.Cid(), msc2.HelloCallback, hg.getHeaviestTipSet, "devnet-staging", "sha2")
	msc2.On("HelloCallback", mock.Anything, mock.Anything, mock.Anything).Return()

	require.NoError(t, mn.LinkAll())
	require.NoError(t, mn.ConnectAllButSelf())

	time.Sleep(time.Millisecond * 50)

	msc1.AssertNumberOfCalls(t, "HelloCallback", 0)
	msc2.AssertNumberOfCalls(t, "HelloCallback", 0)
}

func TestHelloMultiBlock(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mn, err := mocknet.WithNPeers(ctx, 2)
	assert.NoError(t, err)

	a := mn.Hosts()[0]
	b := mn.Hosts()[1]

	genesisA := &types.Block{Nonce: 452}

	heavy1 := th.RequireNewTipSet(t,
		&types.Block{Nonce: 1000, Height: 2},
		&types.Block{Nonce: 1002, Height: 2},
		&types.Block{Nonce: 1004, Height: 2},
	)
	heavy2 := th.RequireNewTipSet(t,
		&types.Block{Nonce: 1001, Height: 3},
		&types.Block{Nonce: 1003, Height: 3},
		&types.Block{Nonce: 1005, Height: 3},
	)

	msc1, msc2 := new(mockHelloCallback), new(mockHelloCallback)
	hg1, hg2 := &mockHeaviestGetter{heavy1}, &mockHeaviestGetter{heavy2}

	New(a, genesisA.Cid(), msc1.HelloCallback, hg1.getHeaviestTipSet, "", "")
	New(b, genesisA.Cid(), msc2.HelloCallback, hg2.getHeaviestTipSet, "", "")

	msc1.On("HelloCallback", b.ID(), heavy2.Key(), uint64(3)).Return()
	msc2.On("HelloCallback", a.ID(), heavy1.Key(), uint64(2)).Return()

	assert.NoError(t, mn.LinkAll())
	assert.NoError(t, mn.ConnectAllButSelf())

	time.Sleep(time.Millisecond * 50)

	msc1.AssertExpectations(t)
	msc2.AssertExpectations(t)
}
