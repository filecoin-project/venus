package discovery_test

import (
	"context"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/util/test"
	"github.com/libp2p/go-libp2p-core/network"
	"testing"
	"time"

	"github.com/filecoin-project/venus/pkg/discovery"
	"github.com/libp2p/go-libp2p-core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	th "github.com/filecoin-project/venus/pkg/testhelpers"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func TestPeerTrackerSelectHead(t *testing.T) {
	tf.UnitTest(t)

	pid0 := th.RequireIntPeerID(t, 0)
	pid1 := th.RequireIntPeerID(t, 1)
	pid2 := th.RequireIntPeerID(t, 2)
	pid3 := th.RequireIntPeerID(t, 3)

	ci0 := types.NewChainInfo(pid0, pid0, th.RequireTipsetWithHeight(t, 6))
	ci1 := types.NewChainInfo(pid1, pid1, th.RequireTipsetWithHeight(t, 10))
	ci2 := types.NewChainInfo(pid2, pid2, th.RequireTipsetWithHeight(t, 7))
	ci3 := types.NewChainInfo(pid3, pid3, th.RequireTipsetWithHeight(t, 9))

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

	ci0 := types.NewChainInfo(pid0, pid0, th.RequireTipsetWithHeight(t, 6))
	ci1 := types.NewChainInfo(pid1, pid1, th.RequireTipsetWithHeight(t, 0))
	ci3 := types.NewChainInfo(pid3, pid3, th.RequireTipsetWithHeight(t, 0))
	ci7 := types.NewChainInfo(pid7, pid7, th.RequireTipsetWithHeight(t, 0))

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

	aCI := types.NewChainInfo(aID, aID, th.RequireTipsetWithHeight(t, 0))
	bCI := types.NewChainInfo(bID, bID, th.RequireTipsetWithHeight(t, 0))

	// self is the tracking node
	// self tracks peers a and b
	// self does not track peer c
	tracker := discovery.NewPeerTracker(peer.ID(""))
	tracker.Track(aCI)
	tracker.Track(bCI)

	// register tracker OnDisconnect callback in self's network
	disconnect := make(chan error)
	notifee := &network.NotifyBundle{}
	notifee.DisconnectedF = func(network network.Network, conn network.Conn) {
		disconnect <- nil
	}
	self.Network().Notify(notifee)

	tracker.RegisterDisconnect(self.Network())

	waitForDisConnect := func() {
		select {
		case <-time.After(time.Second * 5):
			t.Errorf("time out for wait disconnect notify")
		case <-disconnect:
		}
	}

	// disconnect from tracked a and untracked c
	require.NoError(t, mn.DisconnectPeers(selfID, aID))
	waitForDisConnect()
	require.NoError(t, mn.DisconnectPeers(selfID, cID))
	waitForDisConnect()

	time.Sleep(time.Second)
	tracked := tracker.List()
	test.Equal(t, []*types.ChainInfo{bCI}, tracked)
}
