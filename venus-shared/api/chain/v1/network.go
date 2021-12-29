package v1

import (
	"context"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/metrics"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"

	chain2 "github.com/filecoin-project/venus/venus-shared/api/chain"
	"github.com/filecoin-project/venus/venus-shared/libp2p/net"
)

type INetwork interface {
	NetworkGetBandwidthStats(ctx context.Context) metrics.Stats                                    //perm:admin
	NetworkGetPeerAddresses(ctx context.Context) []ma.Multiaddr                                    //perm:admin
	NetworkGetPeerID(ctx context.Context) peer.ID                                                  //perm:admin
	NetworkFindProvidersAsync(ctx context.Context, key cid.Cid, count int) <-chan peer.AddrInfo    //perm:read
	NetworkGetClosestPeers(ctx context.Context, key string) (<-chan peer.ID, error)                //perm:read
	NetworkFindPeer(ctx context.Context, peerID peer.ID) (peer.AddrInfo, error)                    //perm:read
	NetworkConnect(ctx context.Context, addrs []string) (<-chan net.ConnectionResult, error)       //perm:read
	NetworkPeers(ctx context.Context, verbose, latency, streams bool) (*net.SwarmConnInfos, error) //perm:read
	Version(context.Context) (chain2.Version, error)                                               //perm:read
	NetAddrsListen(context.Context) (peer.AddrInfo, error)                                         //perm:read
}
