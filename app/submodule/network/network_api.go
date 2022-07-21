package network

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/p2p/protocol/ping"

	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/metrics"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/venus-shared/api"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
)

var _ v1api.INetwork = &networkAPI{}

type networkAPI struct { //nolint
	network *NetworkSubmodule
}

// NetBandwidthStats gets stats on the current bandwidth usage of the network
func (na *networkAPI) NetBandwidthStats(ctx context.Context) (metrics.Stats, error) {
	return na.network.Network.GetBandwidthStats(), nil
}

// NetBandwidthStatsByPeer returns statistics about the nodes bandwidth
// usage and current rate per peer
func (na *networkAPI) NetBandwidthStatsByPeer(ctx context.Context) (map[string]metrics.Stats, error) {
	return na.network.Network.GetBandwidthStatsByPeer()
}

// NetBandwidthStatsByProtocol returns statistics about the nodes bandwidth
// usage and current rate per protocol
func (na *networkAPI) NetBandwidthStatsByProtocol(ctx context.Context) (map[protocol.ID]metrics.Stats, error) {
	return na.network.Network.GetBandwidthStatsByProtocol()
}

// ID returns the current peer id of the node
func (na *networkAPI) ID(ctx context.Context) (peer.ID, error) {
	return na.network.Network.GetPeerID(), nil
}

// NetFindProvidersAsync issues a findProviders query to the filecoin network content router.
func (na *networkAPI) NetFindProvidersAsync(ctx context.Context, key cid.Cid, count int) <-chan peer.AddrInfo {
	return na.network.Network.Router.FindProvidersAsync(ctx, key, count)
}

// NetGetClosestPeers issues a getClosestPeers query to the filecoin network.
func (na *networkAPI) NetGetClosestPeers(ctx context.Context, key string) ([]peer.ID, error) {
	return na.network.Network.GetClosestPeers(ctx, key)
}

// NetFindPeer searches the libp2p router for a given peer id
func (na *networkAPI) NetFindPeer(ctx context.Context, peerID peer.ID) (peer.AddrInfo, error) {
	return na.network.Network.FindPeer(ctx, peerID)
}

// NetPeerInfo searches the peer info for a given peer id
func (na *networkAPI) NetPeerInfo(ctx context.Context, peerID peer.ID) (*types.ExtendedPeerInfo, error) {
	return na.network.Network.PeerInfo(ctx, peerID)
}

// NetConnect connects to peer at the given address
func (na *networkAPI) NetConnect(ctx context.Context, p peer.AddrInfo) error {
	return na.network.Network.Connect(ctx, p)
}

// NetPeers lists peers currently available on the network
func (na *networkAPI) NetPeers(ctx context.Context) ([]peer.AddrInfo, error) {
	return na.network.Network.Peers(ctx)
}

// NetAgentVersion returns agent version for a given peer id
func (na *networkAPI) NetAgentVersion(ctx context.Context, p peer.ID) (string, error) {
	return na.network.Network.AgentVersion(ctx, p)
}

// NetPing returns result of a ping attempt
func (na *networkAPI) NetPing(ctx context.Context, p peer.ID) (time.Duration, error) {
	result, ok := <-ping.Ping(ctx, na.network.Host, p)
	if !ok {
		return 0, fmt.Errorf("didn't get ping result: %w", ctx.Err())
	}
	return result.RTT, result.Error
}

func (na *networkAPI) Version(context.Context) (types.Version, error) {
	return types.Version{
		Version:    constants.UserVersion(),
		APIVersion: api.FullAPIVersion1,
	}, nil
}

// NetAddrsListen return local p2p address info
func (na *networkAPI) NetAddrsListen(context.Context) (peer.AddrInfo, error) {
	return peer.AddrInfo{
		ID:    na.network.Host.ID(),
		Addrs: na.network.Host.Addrs(),
	}, nil
}

// NetDisconnect disconnect to peer at the given address
func (na *networkAPI) NetDisconnect(_ context.Context, p peer.ID) error {
	return na.network.Network.Disconnect(p)
}

// NetProtectAdd protect peer at the given peers id
func (na *networkAPI) NetProtectAdd(ctx context.Context, peers []peer.ID) error {
	return na.network.Network.ProtectAdd(peers)
}

// NetProtectRemove unprotect peer at the given peers id
func (na *networkAPI) NetProtectRemove(ctx context.Context, peers []peer.ID) error {
	return na.network.Network.ProtectRemove(peers)
}

// NetProtectList returns the peers that are protected
func (na *networkAPI) NetProtectList(ctx context.Context) ([]peer.ID, error) {
	return na.network.Network.ProtectList()
}

// NetConnectedness returns a state signaling connection capabilities
func (na *networkAPI) NetConnectedness(ctx context.Context, p peer.ID) (network.Connectedness, error) {
	return na.network.Network.Connectedness(p)
}

// NetAutoNatStatus return a struct with current NAT status and public dial address
func (na *networkAPI) NetAutoNatStatus(context.Context) (types.NatInfo, error) {
	return na.network.Network.AutoNatStatus()
}
