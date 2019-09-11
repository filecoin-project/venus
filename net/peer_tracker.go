package net

import (
	"context"
	"sort"
	"sync"

	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/filecoin-project/go-filecoin/types"
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
	peers    map[peer.ID]*types.ChainInfo
	trusted  map[peer.ID]struct{}
	updateFn updatePeerFn
}

type updatePeerFn func(ctx context.Context, p peer.ID) (*types.ChainInfo, error)

// NewPeerTracker creates a peer tracker.
func NewPeerTracker(self peer.ID, trust ...peer.ID) *PeerTracker {
	trustedSet := make(map[peer.ID]struct{}, len(trust))
	for _, t := range trust {
		trustedSet[t] = struct{}{}
	}
	return &PeerTracker{
		peers:   make(map[peer.ID]*types.ChainInfo),
		trusted: trustedSet,
		self:    self,
	}
}

// SetUpdateFn sets the update function `f` on the peer tracker. This function is a prerequisite
// to the UpdateTrusted logic.
func (tracker *PeerTracker) SetUpdateFn(f updatePeerFn) {
	tracker.updateFn = f
}

// SelectHead returns the chain info from trusted peers with the greatest height.
// An error is returned if no peers are in the tracker.
func (tracker *PeerTracker) SelectHead() (*types.ChainInfo, error) {
	heads := tracker.listTrusted()
	if len(heads) == 0 {
		return nil, errors.New("no peers tracked")
	}
	sort.Slice(heads, func(i, j int) bool { return heads[i].Height > heads[j].Height })
	return heads[0], nil
}

// UpdateTrusted updates ChainInfo for all trusted peers.
func (tracker *PeerTracker) UpdateTrusted(ctx context.Context) error {
	return tracker.updatePeers(ctx, tracker.trustedPeers()...)
}

// Trust adds `pid` to the peer trackers trusted node set.
func (tracker *PeerTracker) Trust(pid peer.ID) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	tracker.trusted[pid] = struct{}{}
	logPeerTracker.Infof("Trusting peer=%s", pid.Pretty())
}

// Track adds information about a given peer.ID
func (tracker *PeerTracker) Track(ci *types.ChainInfo) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	_, tracking := tracker.peers[ci.Peer]
	_, trusted := tracker.trusted[ci.Peer]
	tracker.peers[ci.Peer] = ci
	logPeerTracker.Infof("Tracking %s, new=%t, count=%d trusted=%t", ci, !tracking, len(tracker.peers), trusted)
}

// Self returns the peer tracker's owner ID
func (tracker *PeerTracker) Self() peer.ID {
	return tracker.self
}

// List returns the chain info of the currently tracked peers (both trusted and untrusted).
// The info tracked by the tracker can change arbitrarily after this is called -- there is no
// guarantee that the peers returned will be tracked when they are used by the caller and no
// guarantee that the chain info is up to date.
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

	_, trusted := tracker.trusted[pid]
	if _, tracking := tracker.peers[pid]; tracking {
		delete(tracker.peers, pid)
		if trusted {
			logPeerTracker.Warningf("Dropping peer=%s trusted=%t", pid.Pretty(), trusted)
		} else {
			logPeerTracker.Infof("Dropping peer=%s trusted=%t", pid.Pretty(), trusted)
		}
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
func (tracker *PeerTracker) listTrusted() []*types.ChainInfo {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	var tracked []*types.ChainInfo
	for p, ci := range tracker.peers {
		if _, trusted := tracker.trusted[p]; trusted {
			tracked = append(tracked, ci)
		}
	}
	out := make([]*types.ChainInfo, len(tracked))
	copy(out, tracked)
	return out
}

// updatePeers will run the trackers updateFn on each peer in `ps` in parallel, iff all updates fail
// is an error is returned, a partial update is considered successful.
func (tracker *PeerTracker) updatePeers(ctx context.Context, ps ...peer.ID) error {
	if tracker.updateFn == nil {
		return errors.New("canot call PeerTracker peer update logic without setting an update function")
	}
	if len(ps) == 0 {
		logPeerTracker.Info("update peers aborting: no peers to update")
		return nil
	}
	var updateErr []error
	grp, ctx := errgroup.WithContext(ctx)
	for _, p := range ps {
		peer := p
		grp.Go(func() error {
			ci, err := tracker.updateFn(ctx, peer)
			if err != nil {
				err = errors.Wrapf(err, "failed to update peer=%s", peer.Pretty())
				updateErr = append(updateErr, err)
				return err
			}
			tracker.Track(ci)
			return nil
		})
	}
	// check if anyone failed to update
	if err := grp.Wait(); err != nil {
		// full failure return an error
		if len(updateErr) == len(ps) {
			logPeerTracker.Errorf("failed to update all %d peers:%v", len(ps), updateErr)
			return errors.New("all peers failed to update")
		}
		// partial failure
		logPeerTracker.Infof("failed to update %d of %d peers:%v", len(updateErr), len(ps), updateErr)
	}
	return nil
}
