package v0

import (
	"context"

	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/metrics"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

type INetwork interface {
	NetworkGetBandwidthStats(ctx context.Context) metrics.Stats                                      //perm:admin
	NetworkGetPeerAddresses(ctx context.Context) []ma.Multiaddr                                      //perm:admin
	NetworkGetPeerID(ctx context.Context) peer.ID                                                    //perm:admin
	NetworkFindProvidersAsync(ctx context.Context, key cid.Cid, count int) <-chan peer.AddrInfo      //perm:read
	NetworkGetClosestPeers(ctx context.Context, key string) ([]peer.ID, error)                       //perm:read
	NetworkFindPeer(ctx context.Context, peerID peer.ID) (peer.AddrInfo, error)                      //perm:read
	NetworkConnect(ctx context.Context, addrs []string) (<-chan types.ConnectionResult, error)       //perm:read
	NetworkPeers(ctx context.Context, verbose, latency, streams bool) (*types.SwarmConnInfos, error) //perm:read
	Version(context.Context) (types.Version, error)                                                  //perm:read
	NetAddrsListen(context.Context) (peer.AddrInfo, error)                                           //perm:read
}
