package v0

import (
	"context"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/metrics"
	network2 "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"

	"github.com/filecoin-project/venus/venus-shared/types"
)

type INetwork interface {
	NetFindProvidersAsync(ctx context.Context, key cid.Cid, count int) <-chan peer.AddrInfo //perm:read
	NetGetClosestPeers(ctx context.Context, key string) ([]peer.ID, error)                  //perm:read
	NetConnectedness(context.Context, peer.ID) (network2.Connectedness, error)              //perm:read
	NetFindPeer(ctx context.Context, p peer.ID) (peer.AddrInfo, error)                      //perm:read
	NetConnect(ctx context.Context, pi peer.AddrInfo) error                                 //perm:admin
	NetPeers(ctx context.Context) ([]peer.AddrInfo, error)                                  //perm:read
	NetPeerInfo(ctx context.Context, p peer.ID) (*types.ExtendedPeerInfo, error)            //perm:read
	NetAgentVersion(ctx context.Context, p peer.ID) (string, error)                         //perm:read
	NetPing(ctx context.Context, p peer.ID) (time.Duration, error)                          //perm:read
	NetAddrsListen(ctx context.Context) (peer.AddrInfo, error)                              //perm:read
	NetDisconnect(ctx context.Context, p peer.ID) error                                     //perm:admin
	NetAutoNatStatus(context.Context) (types.NatInfo, error)                                //perm:read
	Version(ctx context.Context) (types.Version, error)                                     //perm:read
	ID(ctx context.Context) (peer.ID, error)                                                //perm:read

	// NetBandwidthStats returns statistics about the nodes total bandwidth
	// usage and current rate across all peers and protocols.
	NetBandwidthStats(ctx context.Context) (metrics.Stats, error) //perm:read

	// NetBandwidthStatsByPeer returns statistics about the nodes bandwidth
	// usage and current rate per peer
	NetBandwidthStatsByPeer(ctx context.Context) (map[string]metrics.Stats, error) //perm:read

	// NetBandwidthStatsByProtocol returns statistics about the nodes bandwidth
	// usage and current rate per protocol
	NetBandwidthStatsByProtocol(ctx context.Context) (map[protocol.ID]metrics.Stats, error) //perm:read

	NetProtectAdd(ctx context.Context, acl []peer.ID) error    //perm:admin
	NetProtectRemove(ctx context.Context, acl []peer.ID) error //perm:admin
	NetProtectList(ctx context.Context) ([]peer.ID, error)     //perm:read
}
