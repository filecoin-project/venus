package chain

import (
	"fmt"
	"sync"

	peer "github.com/libp2p/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/types"
)

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

type BasicPeerManager struct {
	peerHeadMu sync.Mutex
	peerHeads  map[peer.ID]types.TipSet
}

func NewBasicPeerManager() *BasicPeerManager {
	return &BasicPeerManager{
		peerHeads: make(map[peer.ID]types.TipSet),
	}
}

func (bpm *BasicPeerManager) AddPeer(p peer.ID, h types.TipSet) {
	bpm.peerHeadMu.Lock()
	defer bpm.peerHeadMu.Unlock()

	bpm.peerHeads[p] = h
}

func (bpm *BasicPeerManager) Peers() []peer.ID {
	bpm.peerHeadMu.Lock()
	defer bpm.peerHeadMu.Unlock()

	var out []peer.ID
	for p := range bpm.peerHeads {
		out = append(out, p)
	}
	return out
}

func (bpm *BasicPeerManager) PeerHeads() map[peer.ID]types.TipSet {
	bpm.peerHeadMu.Lock()
	defer bpm.peerHeadMu.Unlock()

	return bpm.peerHeads
}

func (bpm *BasicPeerManager) SelectHead() (types.TipSet, error) {
	bpm.peerHeadMu.Lock()
	defer bpm.peerHeadMu.Unlock()

	if len(bpm.peerHeads) == 0 {
		return types.UndefTipSet, fmt.Errorf("zero peers to select from")
	}
	// for now pick something random?
	for _, h := range bpm.peerHeads {
		return h, nil
	}
	panic("unreachable")
}
