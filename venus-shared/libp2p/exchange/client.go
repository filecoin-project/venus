package exchange

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/venus/venus-shared/chain"
)

// Client is the requesting side of the ChainExchange protocol. It acts as
// a proxy for other components to request chain data from peers. It is chiefly
// used by the Syncer.
type Client interface {
	// GetBlocks fetches block headers from the network, from the provided
	// tipset *backwards*, returning as many tipsets as the count parameter,
	// or less.
	GetBlocks(ctx context.Context, tsk chain.TipSetKey, count int) ([]*chain.TipSet, error)

	// GetChainMessages fetches messages from the network, starting from the first provided tipset
	// and returning messages from as many tipsets as requested or less.
	GetChainMessages(ctx context.Context, tipsets []*chain.TipSet) ([]*CompactedMessages, error)

	// GetFullTipSet fetches a full tipset from a given peer. If successful,
	// the fetched object contains block headers and all messages in full form.
	GetFullTipSet(ctx context.Context, peer []peer.ID, tsk chain.TipSetKey) (*chain.FullTipSet, error)

	// AddPeer adds a peer to the pool of peers that the Client requests
	// data from.
	AddPeer(ctx context.Context, peer peer.ID)

	// RemovePeer removes a peer from the pool of peers that the Client
	// requests data from.
	RemovePeer(ctx context.Context, peer peer.ID)
}
