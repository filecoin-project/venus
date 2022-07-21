package net

import (
	"context"
	"sort"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/metrics"
	network2 "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	swarm "github.com/libp2p/go-libp2p-swarm"
	basichost "github.com/libp2p/go-libp2p/p2p/host/basic"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/venus-shared/types"
)

// Network is a unified interface for dealing with libp2p
type Network struct {
	host    host.Host
	rawHost types.RawHost
	metrics.Reporter
	*Router
}

// New returns a new Network
func New(
	host host.Host,
	rawHost types.RawHost,
	router *Router,
	reporter metrics.Reporter,
) *Network {
	return &Network{
		host:     host,
		rawHost:  rawHost,
		Reporter: reporter,
		Router:   router,
	}
}

// GetPeerAddresses gets the current addresses of the node
func (network *Network) GetPeerAddresses() []ma.Multiaddr {
	return network.host.Addrs()
}

// GetPeerID gets the current peer id from libp2p-host
func (network *Network) GetPeerID() peer.ID {
	return network.host.ID()
}

// GetBandwidthStats gets stats on the current bandwidth usage of the network
func (network *Network) GetBandwidthStats() metrics.Stats {
	return network.Reporter.GetBandwidthTotals()
}

// GetBandwidthStatsByPeer returns statistics about the nodes bandwidth
// usage and current rate per peer
func (network *Network) GetBandwidthStatsByPeer() (map[string]metrics.Stats, error) {
	out := make(map[string]metrics.Stats)
	for p, s := range network.Reporter.GetBandwidthByPeer() {
		out[p.String()] = s
	}
	return out, nil
}

// GetBandwidthStatsByProtocol returns statistics about the nodes bandwidth
// usage and current rate per protocol
func (network *Network) GetBandwidthStatsByProtocol() (map[protocol.ID]metrics.Stats, error) {
	return network.Reporter.GetBandwidthByProtocol(), nil
}

// Connect connects to peer at the given address. Does not retry.
func (network *Network) Connect(ctx context.Context, p peer.AddrInfo) error {
	if swarm, ok := network.host.Network().(*swarm.Swarm); ok {
		swarm.Backoff().Clear(p.ID)
	}
	return network.host.Connect(ctx, p)
}

// Peers lists peers currently available on the network
func (network *Network) Peers(ctx context.Context) ([]peer.AddrInfo, error) {
	if network.host == nil {
		return nil, errors.New("node must be online")
	}

	conns := network.host.Network().Conns()
	peers := make([]peer.AddrInfo, 0, len(conns))
	for _, conn := range conns {
		peers = append(peers, peer.AddrInfo{
			ID:    conn.RemotePeer(),
			Addrs: []ma.Multiaddr{conn.RemoteMultiaddr()},
		})
	}

	return peers, nil
}

// PeerInfo searches the peer info for a given peer id
func (network *Network) PeerInfo(ctx context.Context, p peer.ID) (*types.ExtendedPeerInfo, error) {
	info := &types.ExtendedPeerInfo{ID: p}

	agent, err := network.host.Peerstore().Get(p, "AgentVersion")
	if err == nil {
		info.Agent = agent.(string)
	}

	for _, a := range network.host.Peerstore().Addrs(p) {
		info.Addrs = append(info.Addrs, a.String())
	}
	sort.Strings(info.Addrs)

	protocols, err := network.host.Peerstore().GetProtocols(p)
	if err == nil {
		sort.Strings(protocols)
		info.Protocols = protocols
	}

	if cm := network.host.ConnManager().GetTagInfo(p); cm != nil {
		info.ConnMgrMeta = &types.ConnMgrInfo{
			FirstSeen: cm.FirstSeen,
			Value:     cm.Value,
			Tags:      cm.Tags,
			Conns:     cm.Conns,
		}
	}

	return info, nil
}

// AgentVersion returns agent version for a given peer id
func (network *Network) AgentVersion(ctx context.Context, p peer.ID) (string, error) {
	ag, err := network.host.Peerstore().Get(p, "AgentVersion")
	if err != nil {
		return "", err
	}

	if ag == nil {
		return "unknown", nil
	}

	return ag.(string), nil
}

// Disconnect disconnect to peer at the given address
func (network *Network) Disconnect(p peer.ID) error {
	return network.host.Network().ClosePeer(p)
}

const apiProtectTag = "api"

// ProtectAdd protect peer at the given peers id
func (network *Network) ProtectAdd(peers []peer.ID) error {
	for _, p := range peers {
		network.host.ConnManager().Protect(p, apiProtectTag)
	}

	return nil
}

// ProtectRemove unprotect peer at the given peers id
func (network *Network) ProtectRemove(peers []peer.ID) error {
	for _, p := range peers {
		network.host.ConnManager().Unprotect(p, apiProtectTag)
	}

	return nil
}

// ProtectList returns the peers that are protected
func (network *Network) ProtectList() ([]peer.ID, error) {
	result := make([]peer.ID, 0)
	for _, conn := range network.host.Network().Conns() {
		if network.host.ConnManager().IsProtected(conn.RemotePeer(), apiProtectTag) {
			result = append(result, conn.RemotePeer())
		}
	}

	return result, nil
}

// Connectedness returns a state signaling connection capabilities
func (network *Network) Connectedness(p peer.ID) (network2.Connectedness, error) {
	return network.host.Network().Connectedness(p), nil
}

// AutoNatStatus return a struct with current NAT status and public dial address
func (network *Network) AutoNatStatus() (types.NatInfo, error) {
	autonat := network.rawHost.(*basichost.BasicHost).GetAutoNat()

	if autonat == nil {
		return types.NatInfo{
			Reachability: network2.ReachabilityUnknown,
		}, nil
	}

	var maddr string
	if autonat.Status() == network2.ReachabilityPublic {
		pa, err := autonat.PublicAddr()
		if err != nil {
			return types.NatInfo{}, err
		}
		maddr = pa.String()
	}

	return types.NatInfo{
		Reachability: autonat.Status(),
		PublicAddr:   maddr,
	}, nil
}
