package chain

import (
	"fmt"
	"sync"

	peer "github.com/libp2p/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/types"
)

// PeerManager defines an interface for managin peers and their heads
type PeerManager interface {
	// AddPeer adds a peer to the peer managers store.
	AddPeer(p peer.ID, h types.TipSet)

	// Peers returns the peers the peer manager is aware of.
	Peers() []peer.ID

	// PeerHeads returns all peers and their heads the peer manager knows about.
	PeerHeads() map[peer.ID]types.TipSet

	// Select computes the best head from the peers its aware of.
	SelectHead() (types.TipSet, error)
}

// BasicPeerManager tracks peers and their heads
type BasicPeerManager struct {
	peerHeadMu sync.Mutex
	peerHeads  map[peer.ID]types.TipSet
}

// NewBasicPeerManager returns a BasicPeerManager
func NewBasicPeerManager() *BasicPeerManager {
	return &BasicPeerManager{
		peerHeads: make(map[peer.ID]types.TipSet),
	}
}

// AddPeer adds a peer and its head to the peer manager.
func (bpm *BasicPeerManager) AddPeer(p peer.ID, h types.TipSet) {
	bpm.peerHeadMu.Lock()
	defer bpm.peerHeadMu.Unlock()

	bpm.peerHeads[p] = h
}

// Peers returns the peers the peer manager is aware of.
func (bpm *BasicPeerManager) Peers() []peer.ID {
	bpm.peerHeadMu.Lock()
	defer bpm.peerHeadMu.Unlock()

	var out []peer.ID
	for p := range bpm.peerHeads {
		out = append(out, p)
	}
	return out
}

// PeerHeads returns a list of peers and their heads
func (bpm *BasicPeerManager) PeerHeads() map[peer.ID]types.TipSet {
	bpm.peerHeadMu.Lock()
	defer bpm.peerHeadMu.Unlock()

	return bpm.peerHeads
}

// SelectHead selects the best possible head from the peers and heads its aware of.
func (bpm *BasicPeerManager) SelectHead() (types.TipSet, error) {
	bpm.peerHeadMu.Lock()
	defer bpm.peerHeadMu.Unlock()

	if len(bpm.peerHeads) == 0 {
		return types.UndefTipSet, fmt.Errorf("zero peers to select from")
	}

	// stinky
	var best types.TipSet
	for _, h := range bpm.peerHeads {
		best = h
		break
	}

	if len(bpm.peerHeads) == 1 {
		return best, nil
	}

	// for now pick the head with the heaviest parent.
	for _, h := range bpm.peerHeads {
		maybe, err := h.ParentWeight()
		if err != nil {
			return types.UndefTipSet, err
		}

		bestP, err := best.ParentWeight()
		if err != nil {
			return types.UndefTipSet, err
		}

		if maybe > bestP {
			best = h
		}

	}
	return best, nil
}
