package net

import (
	"fmt"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/go-filecoin/types"
)

// RequestPeerFinder is an interface that returns peers to make fetch requests
// to, and can be told to find a new set of peers when the current peers fail
type RequestPeerFinder interface {
	FindNextPeers() error
	CurrentPeers() []peer.ID
}

// RequestPeerFinderFactory is any function that will return a new peer finder for
// a request with the given originating peer
type RequestPeerFinderFactory func(originatingPeer peer.ID) (RequestPeerFinder, error)

// RequestPeerTracker is an interface that returns information about connected peers
// and their chains (an interface version of PeerTracker)
type RequestPeerTracker interface {
	List() []*types.ChainInfo
	Self() peer.ID
}

// MakeDefaultRequestPeerFinderFactory makes a RequestPeerFinderFactory that
// returns DefaultRequestPeerFinders connected to the given peer tracker
func MakeDefaultRequestPeerFinderFactory(peerTracker RequestPeerTracker) RequestPeerFinderFactory {
	return func(originatingPeer peer.ID) (RequestPeerFinder, error) {
		fetchFromSelf := originatingPeer == peerTracker.Self()
		pri := &DefaultRequestPeerFinder{
			peerTracker: peerTracker,
			triedPeers:  make(map[peer.ID]struct{}),
		}

		// If the new cid triggering this request came from ourselves then
		// the first peer to request from should be ourselves.
		if fetchFromSelf {
			pri.triedPeers[peerTracker.Self()] = struct{}{}
			pri.currentPeer = peerTracker.Self()
			return pri, nil
		}

		// Get a peer ID from the peer tracker
		err := pri.FindNextPeers()
		if err != nil {
			return nil, err
		}
		return pri, nil
	}
}

// DefaultRequestPeerFinder is the default implementation of a request peer finder
// It simply iterates through peers in the peer tracker, one at a time
type DefaultRequestPeerFinder struct {
	peerTracker RequestPeerTracker
	currentPeer peer.ID
	triedPeers  map[peer.ID]struct{}
}

// CurrentPeers returns one or more peers to make the next requests to
func (pri *DefaultRequestPeerFinder) CurrentPeers() []peer.ID {
	return []peer.ID{pri.currentPeer}
}

// FindNextPeers tells the peer finder that the current peers did not successfully
// complete a request and it should look for more
func (pri *DefaultRequestPeerFinder) FindNextPeers() error {
	chains := pri.peerTracker.List()
	for _, chain := range chains {
		if _, tried := pri.triedPeers[chain.Peer]; !tried {
			pri.triedPeers[chain.Peer] = struct{}{}
			pri.currentPeer = chain.Peer
			return nil
		}
	}
	return fmt.Errorf("Unable to find any untried peers")
}
