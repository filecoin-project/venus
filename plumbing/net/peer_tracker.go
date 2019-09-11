package net

import (
	"github.com/libp2p/go-libp2p-core/peer"
)

type peerTracker interface {
	Trust(pid peer.ID)
}

// PeerTrackerProvider provides access to the peers currently being tracked.
type PeerTrackerProvider struct {
	pt peerTracker
}

// NewPeerTrackerProvider returns a new PeerTrackerProvider
func NewPeerTrackerProvider(pt peerTracker) *PeerTrackerProvider {
	return &PeerTrackerProvider{
		pt: pt,
	}
}

// TrustPeer adds `p` to the peer trackers trusted node set.
func (pt *PeerTrackerProvider) TrustPeer(p peer.ID) {
	pt.pt.Trust(p)
}
