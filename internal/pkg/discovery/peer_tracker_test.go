package discovery_test

import (
	"context"
	"sort"
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/discovery"
	"github.com/libp2p/go-libp2p-core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

func TestPeerTrackerTracks(t *testing.T) {
	tf.UnitTest(t)

	tracker := discovery.NewPeerTracker(peer.ID(""))
	pid0 := th.RequireIntPeerID(t, 0)
	pid1 := th.RequireIntPeerID(t, 1)
	pid3 := th.RequireIntPeerID(t, 3)
	pid7 := th.RequireIntPeerID(t, 7)

	ci0 := block.NewChainInfo(pid0, pid0, block.NewTipSetKey(types.CidFromString(t, "somecid")), 6)
	ci1 := block.NewChainInfo(pid1, pid1, block.NewTipSetKey(), 0)
	ci3 := block.NewChainInfo(pid3, pid3, block.NewTipSetKey(), 0)
	ci7 := block.NewChainInfo(pid7, pid7, block.NewTipSetKey(), 0)

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

	ci0 := block.NewChainInfo(pid0, pid0, block.NewTipSetKey(types.CidFromString(t, "somecid0")), 6)
	ci1 := block.NewChainInfo(pid1, pid1, block.NewTipSetKey(types.CidFromString(t, "somecid1")), 10)
	ci2 := block.NewChainInfo(pid2, pid2, block.NewTipSetKey(types.CidFromString(t, "somecid2")), 7)
	ci3 := block.NewChainInfo(pid3, pid3, block.NewTipSetKey(types.CidFromString(t, "somecid3")), 9)

	// trusting pid2 and pid3
	tracker := discovery.NewPeerTracker(pid2, pid3)
	tracker.Track(ci0)
	tracker.Track(ci1)
	tracker.Track(ci2)
	tracker.Track(ci3)

	// select the highest head
	head, err := tracker.SelectHead()
	assert.NoError(t, err)
	assert.Equal(t, head.Head, ci3.Head)
}

func TestPeerTrackerRemove(t *testing.T) {
	tf.UnitTest(t)

	tracker := discovery.NewPeerTracker(peer.ID(""))
	pid0 := th.RequireIntPeerID(t, 0)
	pid1 := th.RequireIntPeerID(t, 1)
	pid3 := th.RequireIntPeerID(t, 3)
	pid7 := th.RequireIntPeerID(t, 7)

	ci0 := block.NewChainInfo(pid0, pid0, block.NewTipSetKey(types.CidFromString(t, "somecid")), 6)
	ci1 := block.NewChainInfo(pid1, pid1, block.NewTipSetKey(), 0)
	ci3 := block.NewChainInfo(pid3, pid3, block.NewTipSetKey(), 0)
	ci7 := block.NewChainInfo(pid7, pid7, block.NewTipSetKey(), 0)

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

	aCI := block.NewChainInfo(aID, aID, block.NewTipSetKey(), 0)
	bCI := block.NewChainInfo(bID, bID, block.NewTipSetKey(), 0)

	// self is the tracking node
	// self tracks peers a and b
	// self does not track peer c
	tracker := discovery.NewPeerTracker(peer.ID(""))
	tracker.Track(aCI)
	tracker.Track(bCI)

	// register tracker OnDisconnect callback in self's network
	tracker.RegisterDisconnect(self.Network())

	// disconnect from tracked a and untracked c
	require.NoError(t, mn.DisconnectPeers(selfID, aID))
	require.NoError(t, mn.DisconnectPeers(selfID, cID))

	tracked := tracker.List()
	assert.Equal(t, []*block.ChainInfo{bCI}, tracked)
}
