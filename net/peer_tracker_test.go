package net_test

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/filecoin-project/go-filecoin/block"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/net"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestPeerTrackerTracks(t *testing.T) {
	tf.UnitTest(t)

	tracker := net.NewPeerTracker(peer.ID(""))
	pid0 := th.RequireIntPeerID(t, 0)
	pid1 := th.RequireIntPeerID(t, 1)
	pid3 := th.RequireIntPeerID(t, 3)
	pid7 := th.RequireIntPeerID(t, 7)

	ci0 := block.NewChainInfo(pid0, block.NewTipSetKey(types.CidFromString(t, "somecid")), 6)
	ci1 := block.NewChainInfo(pid1, block.NewTipSetKey(), 0)
	ci3 := block.NewChainInfo(pid3, block.NewTipSetKey(), 0)
	ci7 := block.NewChainInfo(pid7, block.NewTipSetKey(), 0)

	tracker.Track(ci0)
	tracker.Track(ci1)
	tracker.Track(ci3)
	tracker.Track(ci7)

	tracked := tracker.List()
	sort.Sort(block.CISlice(tracked))
	expected := []*block.ChainInfo{ci0, ci1, ci3, ci7}
	sort.Sort(block.CISlice(expected))
	assert.Equal(t, expected, tracked)

}

func TestPeerTrackerSelectHead(t *testing.T) {
	tf.UnitTest(t)

	pid0 := th.RequireIntPeerID(t, 0)
	pid1 := th.RequireIntPeerID(t, 1)
	pid2 := th.RequireIntPeerID(t, 2)
	pid3 := th.RequireIntPeerID(t, 3)

	ci0 := block.NewChainInfo(pid0, block.NewTipSetKey(types.CidFromString(t, "somecid0")), 6)
	ci1 := block.NewChainInfo(pid1, block.NewTipSetKey(types.CidFromString(t, "somecid1")), 10)
	ci2 := block.NewChainInfo(pid2, block.NewTipSetKey(types.CidFromString(t, "somecid2")), 7)
	ci3 := block.NewChainInfo(pid3, block.NewTipSetKey(types.CidFromString(t, "somecid3")), 9)

	// trusting pid2 and pid3
	tracker := net.NewPeerTracker(pid2, pid3)
	tracker.Track(ci0)
	tracker.Track(ci1)
	tracker.Track(ci2)
	tracker.Track(ci3)

	// select the highest head
	head, err := tracker.SelectHead()
	assert.NoError(t, err)
	assert.Equal(t, head.Head, ci3.Head)
}

func TestPeerTrackerUpdateTrusted(t *testing.T) {
	tf.UnitTest(t)

	pid0 := th.RequireIntPeerID(t, 0)
	pid1 := th.RequireIntPeerID(t, 1)
	pid2 := th.RequireIntPeerID(t, 2)
	pid3 := th.RequireIntPeerID(t, 3)

	// trust pid2 and pid3
	tracker := net.NewPeerTracker(pid3, pid2)

	ci0 := block.NewChainInfo(pid0, block.NewTipSetKey(types.CidFromString(t, "somecid0")), 600)
	ci1 := block.NewChainInfo(pid1, block.NewTipSetKey(types.CidFromString(t, "somecid1")), 10)
	ci2 := block.NewChainInfo(pid2, block.NewTipSetKey(types.CidFromString(t, "somecid2")), 7)
	ci3 := block.NewChainInfo(pid3, block.NewTipSetKey(types.CidFromString(t, "somecid3")), 9)

	tracker.Track(ci0)
	tracker.Track(ci1)
	tracker.Track(ci2)
	tracker.Track(ci3)

	updatedHead := block.NewTipSetKey(types.CidFromString(t, "UPDATE"))
	updatedHeight := uint64(100)
	// update function that changes the tipset and sets height to 100
	tracker.SetUpdateFn(func(ctx context.Context, p peer.ID) (*block.ChainInfo, error) {
		return &block.ChainInfo{
			Head:   updatedHead,
			Height: updatedHeight,
			Peer:   p,
		}, nil
	})

	// update the trusted peers
	assert.NoError(t, tracker.UpdateTrusted(context.Background()))

	tracked := tracker.List()
	assert.Equal(t, 4, len(tracked))
	for i := range tracked {
		if tracked[i].Peer == pid0 || tracked[i].Peer == pid1 {
			assert.NotEqual(t, updatedHead, tracked[i].Head)
			assert.NotEqual(t, updatedHeight, tracked[i].Height)
			assert.False(t, tracked[i].Peer == pid3 || tracked[i].Peer == pid2)
		}
	}

	// Selecting head returns the largest of trusted peers.
	trustHead, err := tracker.SelectHead()
	assert.NoError(t, err)
	assert.Equal(t, updatedHead, trustHead.Head)
	assert.Equal(t, updatedHeight, trustHead.Height)

}

func TestUpdateWithErrors(t *testing.T) {
	tf.UnitTest(t)

	pid0 := th.RequireIntPeerID(t, 0)
	pid1 := th.RequireIntPeerID(t, 1)
	pid2 := th.RequireIntPeerID(t, 2)
	failPeer := th.RequireIntPeerID(t, 3)

	self := peer.ID("")
	trusted := []peer.ID{pid0, pid1, pid2, failPeer}

	// trust them all
	tracker := net.NewPeerTracker(self, trusted...)

	ci0 := block.NewChainInfo(pid0, block.NewTipSetKey(types.CidFromString(t, "somecid0")), 600)
	ci1 := block.NewChainInfo(pid1, block.NewTipSetKey(types.CidFromString(t, "somecid0")), 600)
	ci2 := block.NewChainInfo(pid2, block.NewTipSetKey(types.CidFromString(t, "somecid0")), 600)
	ci3 := block.NewChainInfo(failPeer, block.NewTipSetKey(types.CidFromString(t, "somecid0")), 600)

	tracker.Track(ci0)
	tracker.Track(ci1)
	tracker.Track(ci2)
	tracker.Track(ci3)

	// fail everything
	tracker.SetUpdateFn(func(ctx context.Context, p peer.ID) (*block.ChainInfo, error) {
		return nil, fmt.Errorf("failed to update peer")
	})

	// if it all fails error.
	err := tracker.UpdateTrusted(context.Background())
	assert.Error(t, err, "all updates failed")

	updatedHead := block.NewTipSetKey(types.CidFromString(t, "UPDATE"))
	updatedHeight := uint64(100)

	// fail to update `failPeer`
	tracker.SetUpdateFn(func(ctx context.Context, p peer.ID) (*block.ChainInfo, error) {
		if p == failPeer {
			return nil, fmt.Errorf("failed to update peer")
		}
		return &block.ChainInfo{
			Head:   updatedHead,
			Height: updatedHeight,
			Peer:   p,
		}, nil
	})

	// call update on all peers, there should not be an error for partial failure
	err = tracker.UpdateTrusted(context.Background())
	assert.NoError(t, err, "partial update successful")

	// the peers that didn't error should have an updated head
	cis := tracker.List()
	assert.Equal(t, 4, len(cis))
	for _, ci := range cis {
		if ci.Peer == failPeer {
			assert.Equal(t, ci.Head, ci0.Head)
		} else {
			assert.Equal(t, ci.Head, updatedHead)
		}
	}

}

func TestPeerTrackerRemove(t *testing.T) {
	tf.UnitTest(t)

	tracker := net.NewPeerTracker(peer.ID(""))
	pid0 := th.RequireIntPeerID(t, 0)
	pid1 := th.RequireIntPeerID(t, 1)
	pid3 := th.RequireIntPeerID(t, 3)
	pid7 := th.RequireIntPeerID(t, 7)

	ci0 := block.NewChainInfo(pid0, block.NewTipSetKey(types.CidFromString(t, "somecid")), 6)
	ci1 := block.NewChainInfo(pid1, block.NewTipSetKey(), 0)
	ci3 := block.NewChainInfo(pid3, block.NewTipSetKey(), 0)
	ci7 := block.NewChainInfo(pid7, block.NewTipSetKey(), 0)

	tracker.Track(ci0)
	tracker.Track(ci1)
	tracker.Track(ci3)
	tracker.Track(ci7)

	tracker.Remove(pid1)
	tracker.Remove(pid3)
	tracker.Remove(pid7)

	tracked := tracker.List()
	expected := []*block.ChainInfo{ci0}
	assert.Equal(t, expected, tracked)
}

func TestPeerTrackerNetworkDisconnect(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mn, err := mocknet.FullMeshConnected(ctx, 4)
	require.NoError(t, err)

	self := mn.Hosts()[0]
	a := mn.Hosts()[1]
	b := mn.Hosts()[2]
	c := mn.Hosts()[3]

	selfID := self.ID()
	aID := a.ID()
	bID := b.ID()
	cID := c.ID()

	aCI := block.NewChainInfo(aID, block.NewTipSetKey(), 0)
	bCI := block.NewChainInfo(bID, block.NewTipSetKey(), 0)

	// self is the tracking node
	// self tracks peers a and b
	// self does not track peer c
	tracker := net.NewPeerTracker(peer.ID(""))
	tracker.Track(aCI)
	tracker.Track(bCI)

	// register tracker OnDisconnect callback in self's network
	net.TrackerRegisterDisconnect(self.Network(), tracker)

	// disconnect from tracked a and untracked c
	require.NoError(t, mn.DisconnectPeers(selfID, aID))
	require.NoError(t, mn.DisconnectPeers(selfID, cID))

	tracked := tracker.List()
	assert.Equal(t, []*block.ChainInfo{bCI}, tracked)
}
