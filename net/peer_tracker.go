package net

import (
	"sync"

	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/go-filecoin/types"
)

var logPeerTracker = logging.Logger("peer-tracker")

// PeerTracker is used to record a subset of peers. Its methods are thread safe.
// It is designed to plug directly into libp2p disconnect notifications to
// automatically register dropped connections.
type PeerTracker struct {
	// mu protects peers
	mu sync.RWMutex
	// peers maps peer.IDs to info about their chains
	peers map[peer.ID]*types.ChainInfo
}

// NewPeerTracker creates a peer tracker.
func NewPeerTracker() *PeerTracker {
	return &PeerTracker{
		peers: make(map[peer.ID]*types.ChainInfo),
	}
}

// Track adds information about a given peer.ID
func (tracker *PeerTracker) Track(ci *types.ChainInfo) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	_, tracking := tracker.peers[ci.Peer]
	tracker.peers[ci.Peer] = ci
	logPeerTracker.Infof("Tracking %s, new=%t, count=%d", ci, !tracking, len(tracker.peers))
}

// List returns the chain info of the currently tracked peers.  The info
// tracked by the tracker can change arbitrarily after this is called -- there
// is no guarantee that the peers returned will be tracked when they are used
// by the caller and no guarantee that the chain info is up to date.
func (tracker *PeerTracker) List() []*types.ChainInfo {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	var tracked []*types.ChainInfo
	for _, ci := range tracker.peers {
		tracked = append(tracked, ci)
	}
	out := make([]*types.ChainInfo, len(tracked))
	copy(out, tracked)
	return out
}

// Peers returns a slice of peers the PeerTracker is tracking. The order of the returned slice is
// not determininistic.
func (tracker *PeerTracker) Peers() []peer.ID {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	var peers []peer.ID
	for p := range tracker.peers {
		peers = append(peers, p)
	}
	return peers

}

// Remove removes a peer ID from the tracker.
func (tracker *PeerTracker) Remove(pid peer.ID) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	if _, tracking := tracker.peers[pid]; tracking {
		logPeerTracker.Infof("Dropping peer %s", pid.Pretty())
		delete(tracker.peers, pid)
	}
}

// TrackerRegisterDisconnect registers a tracker remove operation as a libp2p
// "Disconnected" network event callback.
func TrackerRegisterDisconnect(ntwk network.Network, tracker *PeerTracker) {
	notifee := &network.NotifyBundle{}
	notifee.DisconnectedF = func(network network.Network, conn network.Conn) {
		pid := conn.RemotePeer()
		tracker.Remove(pid)
	}
	ntwk.Notify(notifee)
}
