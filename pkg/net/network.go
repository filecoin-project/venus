package net

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/metrics"
	"github.com/libp2p/go-libp2p-core/peer"
	swarm "github.com/libp2p/go-libp2p-swarm"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
)

// Network is a unified interface for dealing with libp2p
type Network struct {
	host host.Host
	metrics.Reporter
	*Router
}

// New returns a new Network
func New(
	host host.Host,
	router *Router,
	reporter metrics.Reporter,
) *Network {
	return &Network{
		host:     host,
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

// Connect connects to peers at the given addresses. Does not retry.
func (network *Network) Connect(ctx context.Context, addrs []string) (<-chan types.ConnectionResult, error) {
	outCh := make(chan types.ConnectionResult)

	swrm, ok := network.host.Network().(*swarm.Swarm)
	if !ok {
		return nil, fmt.Errorf("peerhost network was not a swarm")
	}

	pis, err := PeerAddrsToAddrInfo(addrs)
	if err != nil {
		return nil, err
	}

	go func() {
		var wg sync.WaitGroup
		wg.Add(len(pis))

		for _, pi := range pis {
			go func(pi peer.AddrInfo) {
				swrm.Backoff().Clear(pi.ID)
				err := network.host.Connect(ctx, pi)
				outCh <- types.ConnectionResult{
					PeerID: pi.ID,
					Err:    err,
				}
				wg.Done()
			}(pi)
		}

		wg.Wait()
		close(outCh)
	}()

	return outCh, nil
}

// Peers lists peers currently available on the network
func (network *Network) Peers(ctx context.Context, verbose, latency, streams bool) (*types.SwarmConnInfos, error) {
	if network.host == nil {
		return nil, errors.New("node must be online")
	}

	conns := network.host.Network().Conns()

	out := types.SwarmConnInfos{
		Peers: []types.SwarmConnInfo{},
	}
	for _, c := range conns {
		pid := c.RemotePeer()
		addr := c.RemoteMultiaddr()

		ci := types.SwarmConnInfo{
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
				ci.Streams = append(ci.Streams, types.SwarmStreamInfo{Protocol: string(s.Protocol())})
			}
		}
		sort.Sort(&ci)
		out.Peers = append(out.Peers, ci)
	}

	sort.Sort(&out)
	return &out, nil
}

// Disconnect disconnect to peer at the given address
func (network *Network) Disconnect(p peer.ID) error {
	return network.host.Network().ClosePeer(p)
}
