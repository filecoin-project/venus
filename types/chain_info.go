package types

import (
	"fmt"

	"github.com/libp2p/go-libp2p-core/peer"
)

// ChainInfo is used to track metadata about a peer and its chain.
type ChainInfo struct {
	Peer    peer.ID
	Head    TipSetKey
	Height  uint64
	Trusted bool
}

// NewChainInfo creates a chain info from a peer id a head tipset key and a
// chain height.
func NewChainInfo(peer peer.ID, head TipSetKey, height uint64) *ChainInfo {
	return &ChainInfo{
		Peer:    peer,
		Head:    head,
		Height:  height,
		Trusted: true,
	}
}

// Returns a human-readable string representation of a chain info
func (i *ChainInfo) String() string {
	return fmt.Sprintf("{peer=%s height=%d head=%s}", i.Peer, i.Height, i.Head)
}

// CISlice is for sorting chain infos
type CISlice []*ChainInfo

// Len returns the number of chain infos in the slice.
func (cis CISlice) Len() int { return len(cis) }

// Swap swaps chain infos.
func (cis CISlice) Swap(i, j int) { cis[i], cis[j] = cis[j], cis[i] }

// Less compares chain infos on peer ID.  There should only ever be one chain
// info per peer in a CISlice.
func (cis CISlice) Less(i, j int) bool { return string(cis[i].Peer) < string(cis[j].Peer) }
