package net_test

import (
	"testing"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/net"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestDefaultRequestPeerFinder(t *testing.T) {
	tf.UnitTest(t)
	pid0 := th.RequireIntPeerID(t, 0)
	pid1 := th.RequireIntPeerID(t, 1)
	pid2 := th.RequireIntPeerID(t, 2)
	chain0 := types.NewChainInfo(pid0, types.UndefTipSet.Key(), 0)
	chain1 := types.NewChainInfo(pid1, types.UndefTipSet.Key(), 0)
	chain2 := types.NewChainInfo(pid2, types.UndefTipSet.Key(), 0)

	t.Run("when originating peer is self", func(t *testing.T) {
		fpt := th.NewFakePeerTracker(pid1, chain0, chain1, chain2)
		rpff := net.MakeDefaultRequestPeerFinderFactory(fpt)
		rpf, err := rpff(pid1)
		require.NoError(t, err)
		require.Equal(t, rpf.CurrentPeers(), []peer.ID{pid1})
		err = rpf.FindNextPeers()
		require.NoError(t, err)
		require.Contains(t, []peer.ID{pid0, pid2}, rpf.CurrentPeers()[0])
		err = rpf.FindNextPeers()
		require.NoError(t, err)
		require.Contains(t, []peer.ID{pid0, pid2}, rpf.CurrentPeers()[0])
		err = rpf.FindNextPeers()
		require.EqualError(t, err, "Unable to find any untried peers")
	})

	t.Run("when originating peer is not self", func(t *testing.T) {
		fpt := th.NewFakePeerTracker(pid1, chain0, chain1, chain2)
		rpff := net.MakeDefaultRequestPeerFinderFactory(fpt)
		rpf, err := rpff(pid2)
		require.NoError(t, err)
		require.Contains(t, []peer.ID{pid0, pid1, pid2}, rpf.CurrentPeers()[0])
		err = rpf.FindNextPeers()
		require.NoError(t, err)
		require.Contains(t, []peer.ID{pid0, pid1, pid2}, rpf.CurrentPeers()[0])
		err = rpf.FindNextPeers()
		require.NoError(t, err)
		require.Contains(t, []peer.ID{pid0, pid1, pid2}, rpf.CurrentPeers()[0])
		err = rpf.FindNextPeers()
		require.EqualError(t, err, "Unable to find any untried peers")
	})

	t.Run("when more peers are added to peer tracker", func(t *testing.T) {
		fpt := th.NewFakePeerTracker(pid1, chain0)
		rpff := net.MakeDefaultRequestPeerFinderFactory(fpt)
		rpf, err := rpff(pid2)
		require.NoError(t, err)
		require.Equal(t, []peer.ID{pid0}, rpf.CurrentPeers())
		fpt.SetList(chain1)
		err = rpf.FindNextPeers()
		require.NoError(t, err)
		require.Equal(t, []peer.ID{pid1}, rpf.CurrentPeers())
		fpt.SetList(chain2)
		err = rpf.FindNextPeers()
		require.NoError(t, err)
		require.Equal(t, []peer.ID{pid2}, rpf.CurrentPeers())
		err = rpf.FindNextPeers()
		require.EqualError(t, err, "Unable to find any untried peers")
	})

	t.Run("when no peers are available", func(t *testing.T) {
		fpt := th.NewFakePeerTracker(pid1)
		rpff := net.MakeDefaultRequestPeerFinderFactory(fpt)
		rpf, err := rpff(pid2)
		require.Nil(t, rpf)
		require.EqualError(t, err, "Unable to find any untried peers")
	})
}
