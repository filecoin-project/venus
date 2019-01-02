package impl

import (
	"context"
	"fmt"
	"sort"

	"gx/ipfs/QmPiemjiKBC9VA7vZF82m4x1oygtg2c2YVqag8PX7dN1BD/go-libp2p-peerstore"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	peer "gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	swarm "gx/ipfs/QmegQFxhr1J6yZ1vDQuDmJi5jntmj6BL96S11HVtXNCaHb/go-libp2p-swarm"

	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/filnet"
)

type nodeSwarm struct {
	api *nodeAPI
}

// COPIED FROM go-ipfs core/commands/swarm.go
// TODO a lot of this functionality should migrate to the filnet package.

func newNodeSwarm(api *nodeAPI) *nodeSwarm {
	return &nodeSwarm{api: api}
}

func (ns *nodeSwarm) Peers(ctx context.Context, verbose, latency, streams bool) (*api.SwarmConnInfos, error) {
	nd := ns.api.node

	if nd.Host() == nil {
		return nil, ErrNodeOffline
	}

	conns := nd.Host().Network().Conns()

	var out api.SwarmConnInfos
	for _, c := range conns {
		pid := c.RemotePeer()
		addr := c.RemoteMultiaddr()

		ci := api.SwarmConnInfo{
			Addr: addr.String(),
			Peer: pid.Pretty(),
		}

		/* FIXME(steb)
		swcon, ok := c.(*swarm.Conn)
		if ok {
			ci.Muxer = fmt.Sprintf("%T", swcon.StreamConn().Conn())
		}
		*/

		if verbose || latency {
			lat := nd.Host().Peerstore().LatencyEWMA(pid)
			if lat == 0 {
				ci.Latency = "n/a"
			} else {
				ci.Latency = lat.String()
			}
		}
		if verbose || streams {
			strs := c.GetStreams()

			for _, s := range strs {
				ci.Streams = append(ci.Streams, api.SwarmStreamInfo{Protocol: string(s.Protocol())})
			}
		}
		sort.Sort(&ci)
		out.Peers = append(out.Peers, ci)
	}

	sort.Sort(&out)
	return &out, nil
}

func (ns *nodeSwarm) Connect(ctx context.Context, addrs []string) ([]api.SwarmConnectResult, error) {
	nd := ns.api.node

	swrm, ok := nd.Host().Network().(*swarm.Swarm)
	if !ok {
		return nil, fmt.Errorf("peerhost network was not a swarm")
	}

	pis, err := filnet.PeerAddrsToPeerInfos(addrs)
	if err != nil {
		return nil, err
	}

	output := make([]api.SwarmConnectResult, len(pis))
	for i, pi := range pis {
		swrm.Backoff().Clear(pi.ID)

		output[i].Peer = pi.ID.Pretty()

		if err := nd.Host().Connect(ctx, pi); err != nil {
			return nil, errors.Wrapf(err, "peer: %s", output[i].Peer)
		}
	}

	return output, nil
}

func (ns *nodeSwarm) FindPeer(ctx context.Context, peerID peer.ID) (peerstore.PeerInfo, error) {
	return ns.api.node.Router.FindPeer(ctx, peerID)
}
