package v0api

import (
	"context"

	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/metrics"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

type INetwork interface {
	// Rule[perm:admin]
	NetworkGetBandwidthStats(ctx context.Context) metrics.Stats
	// Rule[perm:admin]
	NetworkGetPeerAddresses(ctx context.Context) []ma.Multiaddr
	// Rule[perm:admin]
	NetworkGetPeerID(ctx context.Context) peer.ID
	// Rule[perm:read]
	NetworkFindProvidersAsync(ctx context.Context, key cid.Cid, count int) <-chan peer.AddrInfo
	// Rule[perm:admin]
	NetworkGetClosestPeers(ctx context.Context, key string) ([]peer.ID, error)
	// Rule[perm:read]
	NetworkFindPeer(ctx context.Context, peerID peer.ID) (peer.AddrInfo, error)
	// Rule[perm:read]
	NetworkConnect(ctx context.Context, addrs []string) (<-chan types.ConnectionResult, error)
	// Rule[perm:read]
	NetworkPeers(ctx context.Context, verbose, latency, streams bool) (*types.SwarmConnInfos, error)
	// Rule[perm:read]
	Version(context.Context) (types.Version, error)
	// Rule[perm:read]
	NetAddrsListen(context.Context) (peer.AddrInfo, error)
}
