package net

import (
	"context"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	pstore "gx/ipfs/QmRhFARzTHcFh8wUxwN5KvyTGq73FLC65EfFAhz8Ng7aGb/go-libp2p-peerstore"
	routing "gx/ipfs/QmWaDSNoSdSXU9b6udyaq9T8y6LkzMwqWxECznFqvtcTsk/go-libp2p-routing"
)

// This struct wraps the filecoin nodes router.  This router is a
// go-libp2p-routing.IpfsRouting interface that provides both PeerRouting,
// ContentRouting and a Bootstrap init process. Filecoin nodes in online mode
// use a go-libp2p-kad-dht DHT to satisfy this interface. Nodes run the
// Bootstrap function to join the DHT on start up. The PeerRouting functionality
// enables filecoin nodes to lookup the network addresses of their peers given a
// peerID.  The ContentRouting functionality enables peers to provide and
// discover providers of network services. This is currently used by the
// auto-relay feature in the filecoin network to allow nodes to advertise
// themselves as relay nodes and discover other relay nodes.
//
// The IpfsRouting interface and its DHT instantiation also carries ValueStore
// functionality for using the DHT as a key value store.  Filecoin nodes do
// not currently use this functionality.

// Router exposes the methods on the internal filecoin router that are needed
// by the system plumbing API.
type Router struct {
	routing routing.IpfsRouting
}

// NewRouter builds a new router.
func NewRouter(r routing.IpfsRouting) *Router {
	return &Router{routing: r}
}

// FindProvidersAsync searches for and returns peers who are able to provide a
// given key.
func (r *Router) FindProvidersAsync(ctx context.Context, key cid.Cid, count int) <-chan pstore.PeerInfo {
	return r.routing.FindProvidersAsync(ctx, key, count)
}
