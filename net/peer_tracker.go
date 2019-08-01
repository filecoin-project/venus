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
	// peers maps stringified peer.IDs to info about their chains
	peers map[string]*types.ChainInfo
}

// NewPeerTracker creates a peer tracker.
func NewPeerTracker() *PeerTracker {
	return &PeerTracker{
		peers: make(map[string]*types.ChainInfo),
	}
}

// Track adds information about a given peer.ID
func (tracker *PeerTracker) Track(ci *types.ChainInfo) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	pidKey := peer.IDB58Encode(ci.Peer)
	if _, tracking := tracker.peers[pidKey]; tracking {
		logPeerTracker.Warningf("unexpected duplicate track on peer: %s", pidKey)
	}
	tracker.peers[pidKey] = ci
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

// Remove removes a peer ID from the tracker.
func (tracker *PeerTracker) Remove(pid peer.ID) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	pidKey := peer.IDB58Encode(pid)
	if _, tracking := tracker.peers[pidKey]; tracking {
		delete(tracker.peers, pidKey)
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
