package types

import (
	"fmt"

	"github.com/libp2p/go-libp2p-core/peer"
)

// ChainInfo is used to track metadata about a peer and its chain.
type ChainInfo struct {
	// The originator of the TipSetKey propagation wave.
	Source peer.ID
	// The peer that sent us the TipSetKey message.
	Sender peer.ID
	Head   *TipSet
}

// NewChainInfo creates a chain info from a peer id a head tipset key and a
// chain height.
func NewChainInfo(source peer.ID, sender peer.ID, head *TipSet) *ChainInfo {
	return &ChainInfo{
		Source: source,
		Sender: sender,
		Head:   head,
	}
}

// Returns a human-readable string representation of a chain info
func (i *ChainInfo) String() string {
	return fmt.Sprintf("{source=%s sender:%s height=%d head=%s}", i.Source, i.Sender, i.Head.Height(), i.Head.Key())
}
