package network

import (
	"context"
	"github.com/filecoin-project/venus/app/submodule/apiface"
	"github.com/filecoin-project/venus/app/submodule/apitypes"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/net"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/metrics"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

var _ apiface.INetwork = &networkAPI{}

type networkAPI struct { //nolint
	network *NetworkSubmodule
}

// NetworkGetBandwidthStats gets stats on the current bandwidth usage of the network
func (na *networkAPI) NetworkGetBandwidthStats(ctx context.Context) metrics.Stats {
	return na.network.Network.GetBandwidthStats()
}

// NetworkGetPeerAddresses gets the current addresses of the node
func (na *networkAPI) NetworkGetPeerAddresses(ctx context.Context) []ma.Multiaddr {
	return na.network.Network.GetPeerAddresses()
}

// NetworkGetPeerID gets the current peer id of the node
func (na *networkAPI) NetworkGetPeerID(ctx context.Context) peer.ID {
	return na.network.Network.GetPeerID()
}

// NetworkFindProvidersAsync issues a findProviders query to the filecoin network content router.
func (na *networkAPI) NetworkFindProvidersAsync(ctx context.Context, key cid.Cid, count int) <-chan peer.AddrInfo {
	return na.network.Network.Router.FindProvidersAsync(ctx, key, count)
}

// NetworkGetClosestPeers issues a getClosestPeers query to the filecoin network.
func (na *networkAPI) NetworkGetClosestPeers(ctx context.Context, key string) (<-chan peer.ID, error) {
	return na.network.Network.GetClosestPeers(ctx, key)
}

// NetworkFindPeer searches the libp2p router for a given peer id
func (na *networkAPI) NetworkFindPeer(ctx context.Context, peerID peer.ID) (peer.AddrInfo, error) {
	return na.network.Network.FindPeer(ctx, peerID)
}

// NetworkConnect connects to peers at the given addresses
func (na *networkAPI) NetworkConnect(ctx context.Context, addrs []string) (<-chan net.ConnectionResult, error) {
	return na.network.Network.Connect(ctx, addrs)
}

// NetworkPeers lists peers currently available on the network
func (na *networkAPI) NetworkPeers(ctx context.Context, verbose, latency, streams bool) (*net.SwarmConnInfos, error) {
	return na.network.Network.Peers(ctx, verbose, latency, streams)
}

func (na *networkAPI) Version(context.Context) (apitypes.Version, error) {
	return apitypes.Version{
		Version:    constants.UserVersion(),
		APIVersion: constants.FullAPIVersion1,
	}, nil
}

func (na *networkAPI) NetAddrsListen(context.Context) (peer.AddrInfo, error) {
	return peer.AddrInfo{
		ID:    na.network.Host.ID(),
		Addrs: na.network.Host.Addrs(),
	}, nil
}
