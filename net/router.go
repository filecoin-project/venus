package net

import (
	"context"
	"errors"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	"github.com/libp2p/go-libp2p-kad-dht"
)

// This struct wraps the filecoin nodes router.  This router is a
// go-libp2p-core/routing.Routing interface that provides both PeerRouting,
// ContentRouting and a Bootstrap init process. Filecoin nodes in online mode
// use a go-libp2p-kad-dht DHT to satisfy this interface. Nodes run the
// Bootstrap function to join the DHT on start up. The PeerRouting functionality
// enables filecoin nodes to lookup the network addresses of their peers given a
// peerID.  The ContentRouting functionality enables peers to provide and
// discover providers of network services. This is currently used by the
// auto-relay feature in the filecoin network to allow nodes to advertise
// themselves as relay nodes and discover other relay nodes.
//
// The Routing interface and its DHT instantiation also carries ValueStore
// functionality for using the DHT as a key value store.  Filecoin nodes do
// not currently use this functionality.

// Router exposes the methods on the internal filecoin router that are needed
// by the system plumbing API.
type Router struct {
	routing routing.Routing
}

// NewRouter builds a new router.
func NewRouter(r routing.Routing) *Router {
	return &Router{routing: r}
}

// FindProvidersAsync searches for and returns peers who are able to provide a
// given key.
func (r *Router) FindProvidersAsync(ctx context.Context, key cid.Cid, count int) <-chan peer.AddrInfo {
	return r.routing.FindProvidersAsync(ctx, key, count)
}

// FindPeer searches the libp2p router for a given peer id
func (r *Router) FindPeer(ctx context.Context, peerID peer.ID) (peer.AddrInfo, error) {
	return r.routing.FindPeer(ctx, peerID)
}

// GetClosestPeers returns a channel of the K closest peers  to the given key,
// K is the 'K Bucket' parameter of the Kademlia DHT protocol.
func (r *Router) GetClosestPeers(ctx context.Context, key string) (<-chan peer.ID, error) {
	ipfsDHT, ok := r.routing.(*dht.IpfsDHT)
	if !ok {
		return nil, errors.New("underlying routing should be pointer of IpfsDHT")
	}
	return ipfsDHT.GetClosestPeers(ctx, key)
}
