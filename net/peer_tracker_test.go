package net_test

import (
	"context"
	"sort"
	"testing"

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

	tracker := net.NewPeerTracker()
	pid0 := th.RequireIntPeerID(t, 0)
	pid1 := th.RequireIntPeerID(t, 1)
	pid3 := th.RequireIntPeerID(t, 3)
	pid7 := th.RequireIntPeerID(t, 7)

	ci0 := types.NewChainInfo(pid0, types.NewTipSetKey(types.CidFromString(t, "somecid")), 6)
	ci1 := types.NewChainInfo(pid1, types.NewTipSetKey(), 0)
	ci3 := types.NewChainInfo(pid3, types.NewTipSetKey(), 0)
	ci7 := types.NewChainInfo(pid7, types.NewTipSetKey(), 0)

	tracker.Track(ci0)
	tracker.Track(ci1)
	tracker.Track(ci3)
	tracker.Track(ci7)

	tracked := tracker.List()
	sort.Sort(types.CISlice(tracked))
	expected := []*types.ChainInfo{ci0, ci1, ci3, ci7}
	sort.Sort(types.CISlice(expected))
	assert.Equal(t, expected, tracked)
}

func TestPeerTrackerRemove(t *testing.T) {
	tf.UnitTest(t)

	tracker := net.NewPeerTracker()
	pid0 := th.RequireIntPeerID(t, 0)
	pid1 := th.RequireIntPeerID(t, 1)
	pid3 := th.RequireIntPeerID(t, 3)
	pid7 := th.RequireIntPeerID(t, 7)

	ci0 := types.NewChainInfo(pid0, types.NewTipSetKey(types.CidFromString(t, "somecid")), 6)
	ci1 := types.NewChainInfo(pid1, types.NewTipSetKey(), 0)
	ci3 := types.NewChainInfo(pid3, types.NewTipSetKey(), 0)
	ci7 := types.NewChainInfo(pid7, types.NewTipSetKey(), 0)

	tracker.Track(ci0)
	tracker.Track(ci1)
	tracker.Track(ci3)
	tracker.Track(ci7)

	tracker.Remove(pid1)
	tracker.Remove(pid3)
	tracker.Remove(pid7)

	tracked := tracker.List()
	expected := []*types.ChainInfo{ci0}
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

	aCI := types.NewChainInfo(aID, types.NewTipSetKey(), 0)
	bCI := types.NewChainInfo(bID, types.NewTipSetKey(), 0)

	// self is the tracking node
	// self tracks peers a and b
	// self does not track peer c
	tracker := net.NewPeerTracker()
	tracker.Track(aCI)
	tracker.Track(bCI)

	// register tracker OnDisconnect callback in self's network
	net.TrackerRegisterDisconnect(self.Network(), tracker)

	// disconnect from tracked a and untracked c
	require.NoError(t, mn.DisconnectPeers(selfID, aID))
	require.NoError(t, mn.DisconnectPeers(selfID, cID))

	tracked := tracker.List()
	assert.Equal(t, []*types.ChainInfo{bCI}, tracked)
}
