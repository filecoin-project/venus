package discovery

import (
	"sort"
	"sync"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
)

var logPeerTracker = logging.Logger("peer-tracker")

// PeerTracker is used to record a subset of peers. Its methods are thread safe.
// It is designed to plug directly into libp2p disconnect notifications to
// automatically register dropped connections.
type PeerTracker struct {
	// mu protects peers
	mu sync.RWMutex

	// self tracks the ID of the peer tracker's owner
	self peer.ID

	// peers maps peer.IDs to info about their chains
	peers   map[peer.ID]*block.ChainInfo
	trusted map[peer.ID]struct{}
}

// NewPeerTracker creates a peer tracker.
func NewPeerTracker(self peer.ID, trust ...peer.ID) *PeerTracker {
	trustedSet := make(map[peer.ID]struct{}, len(trust))
	for _, t := range trust {
		trustedSet[t] = struct{}{}
	}
	return &PeerTracker{
		peers:   make(map[peer.ID]*block.ChainInfo),
		trusted: trustedSet,
		self:    self,
	}
}

// SelectHead returns the chain info from trusted peers with the greatest height.
// An error is returned if no peers are in the tracker.
func (tracker *PeerTracker) SelectHead() (*block.ChainInfo, error) {
	heads := tracker.listTrusted()
	if len(heads) == 0 {
		return nil, errors.New("no peers tracked")
	}
	sort.Slice(heads, func(i, j int) bool { return heads[i].Height > heads[j].Height })
	return heads[0], nil
}

// Track adds information about a given peer.ID
func (tracker *PeerTracker) Track(ci *block.ChainInfo) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	_, tracking := tracker.peers[ci.Sender]
	_, trusted := tracker.trusted[ci.Sender]
	tracker.peers[ci.Sender] = ci
	logPeerTracker.Infow("Track peer", "chainInfo", ci, "new", !tracking, "count", len(tracker.peers), "trusted", trusted)
}

// Self returns the peer tracker's owner ID
func (tracker *PeerTracker) Self() peer.ID {
	return tracker.self
}

// List returns the chain info of the currently tracked peers (both trusted and untrusted).
// The info tracked by the tracker can change arbitrarily after this is called -- there is no
// guarantee that the peers returned will be tracked when they are used by the caller and no
// guarantee that the chain info is up to date.
func (tracker *PeerTracker) List() []*block.ChainInfo {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	var tracked []*block.ChainInfo
	for _, ci := range tracker.peers {
		tracked = append(tracked, ci)
	}
	out := make([]*block.ChainInfo, len(tracked))
	copy(out, tracked)
	return out
}

// Remove removes a peer ID from the tracker.
func (tracker *PeerTracker) Remove(pid peer.ID) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	_, trusted := tracker.trusted[pid]
	if _, tracking := tracker.peers[pid]; tracking {
		delete(tracker.peers, pid)
		if trusted {
			logPeerTracker.Warnw("Dropping peer", "peer", pid.Pretty(), "trusted", trusted)
		} else {
			logPeerTracker.Infow("Dropping peer", "peer", pid.Pretty(), "trusted", trusted)
		}
	}
}

// RegisterDisconnect registers a tracker remove operation as a libp2p
// "Disconnected" network event callback.
func (tracker *PeerTracker) RegisterDisconnect(ntwk network.Network) {
	notifee := &network.NotifyBundle{}
	notifee.DisconnectedF = func(network network.Network, conn network.Conn) {
		pid := conn.RemotePeer()
		tracker.Remove(pid)
	}
	ntwk.Notify(notifee)
}

// trustedPeers returns a slice of peers trusted by the PeerTracker. trustedPeers remain constant after
// the PeerTracker has been initialized.
func (tracker *PeerTracker) trustedPeers() []peer.ID {
	var peers []peer.ID
	for p := range tracker.trusted {
		peers = append(peers, p)
	}
	return peers
}

// listTrusted returns the chain info of the trusted tracked peers. The info tracked by the tracker can
// change arbitrarily after this is called -- there is no guarantee that the peers returned will be
// tracked when they are used by the caller and no guarantee that the chain info is up to date.
func (tracker *PeerTracker) listTrusted() []*block.ChainInfo {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	var tracked []*block.ChainInfo
	for p, ci := range tracker.peers {
		if _, trusted := tracker.trusted[p]; trusted {
			tracked = append(tracked, ci)
		}
	}
	out := make([]*block.ChainInfo, len(tracked))
	copy(out, tracked)
	return out
}
