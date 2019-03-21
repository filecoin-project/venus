package net

import (
	"context"
	"fmt"
	"sort"

	ma "gx/ipfs/QmNTCey11oxhb1AxDnQBRHtdhap6Ctud872NjAYPYYXPuc/go-multiaddr"
	"gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"
	"gx/ipfs/QmU7iTrsNaJfu1Rf5DrvaJLH9wJtQwmP4Dj8oPduprAU68/go-libp2p-swarm"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmZZseAa9xcK6tT3YpaShNUAEpyRAoWmUL5ojH3uGNepAc/go-libp2p-metrics"
	"gx/ipfs/QmcNGX5RaxPPCYwa6yGXM1EcUbrreTTinixLcYGmMwf1sx/go-libp2p/p2p/protocol/ping"
	"gx/ipfs/Qmd52WKRSwrBK5gUaJKawryZQ5by6UbNB8KVW2Zy6JtbyW/go-libp2p-host"

	"github.com/filecoin-project/go-filecoin/net/pubsub"
)

// SwarmConnInfo represents details about a single swarm connection.
type SwarmConnInfo struct {
	Addr    string
	Peer    string
	Latency string
	Muxer   string
	Streams []SwarmStreamInfo
}

// SwarmStreamInfo represents details about a single swarm stream.
type SwarmStreamInfo struct {
	Protocol string
}

func (ci *SwarmConnInfo) Less(i, j int) bool {
	return ci.Streams[i].Protocol < ci.Streams[j].Protocol
}

func (ci *SwarmConnInfo) Len() int {
	return len(ci.Streams)
}

func (ci *SwarmConnInfo) Swap(i, j int) {
	ci.Streams[i], ci.Streams[j] = ci.Streams[j], ci.Streams[i]
}

// SwarmConnInfos represent details about a list of swarm connections.
type SwarmConnInfos struct {
	Peers []SwarmConnInfo
}

func (ci SwarmConnInfos) Less(i, j int) bool {
	return ci.Peers[i].Addr < ci.Peers[j].Addr
}

func (ci SwarmConnInfos) Len() int {
	return len(ci.Peers)
}

func (ci SwarmConnInfos) Swap(i, j int) {
	ci.Peers[i], ci.Peers[j] = ci.Peers[j], ci.Peers[i]
}

// Network is a unified interface for dealing with libp2p
type Network struct {
	host host.Host
	*pubsub.Subscriber
	*pubsub.Publisher
	metrics.Reporter
	*Router
	*ping.PingService
}

// New returns a new Network
func New(
	host host.Host,
	publisher *pubsub.Publisher,
	subscriber *pubsub.Subscriber,
	router *Router,
	reporter metrics.Reporter,
	pinger *ping.PingService,
) *Network {
	return &Network{
		host:        host,
		PingService: pinger,
		Publisher:   publisher,
		Reporter:    reporter,
		Router:      router,
		Subscriber:  subscriber,
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

// ConnectionResult represents the result of an attempted connection from the
// Connect method
type ConnectionResult struct {
	PeerID peer.ID
	Err    error
}

// Connect connects to peers at the given addresses. Does not retry, and does not
// try to connect to any more peers if any connection fails.
func (network *Network) Connect(ctx context.Context, addrs []string) (<-chan ConnectionResult, error) {
	out := make(chan ConnectionResult)

	swrm, ok := network.host.Network().(*swarm.Swarm)
	if !ok {
		return nil, fmt.Errorf("peerhost network was not a swarm")
	}

	pis, err := PeerAddrsToPeerInfos(addrs)
	if err != nil {
		return nil, err
	}

	go func() {
		for _, pi := range pis {
			swrm.Backoff().Clear(pi.ID)
			err := network.host.Connect(ctx, pi)
			out <- ConnectionResult{
				PeerID: pi.ID,
				Err:    err,
			}
		}

		close(out)
	}()

	return out, nil
}

// Peers lists peers currently available on the network
func (network *Network) Peers(ctx context.Context, verbose, latency, streams bool) (*SwarmConnInfos, error) {
	if network.host == nil {
		return nil, errors.New("node must be online")
	}

	conns := network.host.Network().Conns()

	var out SwarmConnInfos
	for _, c := range conns {
		pid := c.RemotePeer()
		addr := c.RemoteMultiaddr()

		ci := SwarmConnInfo{
			Addr: addr.String(),
			Peer: pid.Pretty(),
		}

		if verbose || latency {
			lat := network.host.Peerstore().LatencyEWMA(pid)
			if lat == 0 {
				ci.Latency = "n/a"
			} else {
				ci.Latency = lat.String()
			}
		}
		if verbose || streams {
			strs := c.GetStreams()

			for _, s := range strs {
				ci.Streams = append(ci.Streams, SwarmStreamInfo{Protocol: string(s.Protocol())})
			}
		}
		sort.Sort(&ci)
		out.Peers = append(out.Peers, ci)
	}

	sort.Sort(&out)
	return &out, nil
}
