package core

import (
	"context"
	"testing"
	"time"

	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	mocknet "gx/ipfs/QmZ86eLPtXkQ1Dfa992Q8NpXArUoWWh3y728JDcWvzRrvC/go-libp2p/p2p/net/mock"
	peer "gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	types "github.com/filecoin-project/go-filecoin/types"
)

type mockSyncCallback struct {
	mock.Mock
}

func (msb *mockSyncCallback) SyncCallback(p peer.ID, c *cid.Cid, h uint64) {
	msb.Called(p, c, h)
}

type mockBestGetter struct {
	best *types.Block
}

func (mbg *mockBestGetter) getBestBlock() *types.Block {
	return mbg.best
}

func TestHelloHandshake(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mn, err := mocknet.WithNPeers(ctx, 2)
	assert.NoError(t, err)

	a := mn.Hosts()[0]
	b := mn.Hosts()[1]

	genesisA := &types.Block{Nonce: 451}

	best1 := &types.Block{Nonce: 1000, Height: 2}
	best2 := &types.Block{Nonce: 1001, Height: 3}

	msc1, msc2 := new(mockSyncCallback), new(mockSyncCallback)
	bg1, bg2 := &mockBestGetter{best1}, &mockBestGetter{best2}

	h1 := NewHello(a, genesisA.Cid(), msc1.SyncCallback, bg1.getBestBlock)
	h2 := NewHello(b, genesisA.Cid(), msc2.SyncCallback, bg2.getBestBlock)
	_, _ = h1, h2

	msc1.On("SyncCallback", b.ID(), best2.Cid(), uint64(3)).Return()
	msc2.On("SyncCallback", a.ID(), best1.Cid(), uint64(2)).Return()

	mn.LinkAll()
	mn.ConnectAllButSelf()

	time.Sleep(time.Millisecond * 50)

	msc1.AssertExpectations(t)
	msc2.AssertExpectations(t)
}

func TestHelloBadGenesis(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mn, err := mocknet.WithNPeers(ctx, 2)
	assert.NoError(t, err)

	a := mn.Hosts()[0]
	b := mn.Hosts()[1]

	genesisA := &types.Block{Nonce: 451}
	genesisB := &types.Block{Nonce: 101}

	best1 := &types.Block{Nonce: 1000, Height: 2}
	best2 := &types.Block{Nonce: 1001, Height: 3}

	msc1, msc2 := new(mockSyncCallback), new(mockSyncCallback)
	bg1, bg2 := &mockBestGetter{best1}, &mockBestGetter{best2}

	h1 := NewHello(a, genesisA.Cid(), msc1.SyncCallback, bg1.getBestBlock)
	h2 := NewHello(b, genesisB.Cid(), msc2.SyncCallback, bg2.getBestBlock)
	_, _ = h1, h2

	msc1.On("SyncCallback", mock.Anything, mock.Anything, mock.Anything).Return()
	msc2.On("SyncCallback", mock.Anything, mock.Anything, mock.Anything).Return()

	mn.LinkAll()
	mn.ConnectAllButSelf()

	time.Sleep(time.Millisecond * 50)

	msc1.AssertNumberOfCalls(t, "SyncCallback", 0)
	msc2.AssertNumberOfCalls(t, "SyncCallback", 0)
}
