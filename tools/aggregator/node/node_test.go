package aggregator

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"golang.org/x/sync/errgroup"
	"testing"

	host "gx/ipfs/QmPMtD39NN63AEUNghk1LFQcTLcCmYL8MtRzdv8BRUsC4Z/go-libp2p-host"
	crypto "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	net "gx/ipfs/QmQSbtGXCyNrj34LWL8EgXyNNYDZ8r3SwQcpW5pPxVhLnM/go-libp2p-net"
	peer "gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	libp2p "gx/ipfs/QmVM6VuGaWcAaYjxG2om6XxMmpP3Rt9rw4nbMXVNYAPLhS/go-libp2p"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
	pstore "gx/ipfs/QmeKD8YT7887Xu6Z86iZmpYNxrLogJexqxEugSmaf14k64/go-libp2p-peerstore"

	fcmetrics "github.com/filecoin-project/go-filecoin/metrics"

	"github.com/stretchr/testify/require"
)

type heartbeatNode struct {
	Host host.Host
	Beat func() fcmetrics.Heartbeat
}

func newHeartbeatForTestGetter() func() fcmetrics.Heartbeat {
	i := 0
	return func() fcmetrics.Heartbeat {
		i++
		return fcmetrics.Heartbeat{
			Tipset:   fmt.Sprintf("tipset%d", i),
			Height:   uint64(i),
			Nickname: "MockBeat",
		}
	}
}

func newHeartbeatNode(t *testing.T) *heartbeatNode {
	require := require.New(t)

	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	require.NoError(err)

	opts := []libp2p.Option{
		libp2p.Identity(priv),
	}

	simple, err := libp2p.New(context.Background(), opts...)
	require.NoError(err)
	return &heartbeatNode{
		Host: simple,
		Beat: newHeartbeatForTestGetter(),
	}
}

func (s heartbeatNode) StartAndConnect(t *testing.T, maddr ma.Multiaddr) net.Stream {
	require := require.New(t)

	// from libp2p echo example
	pid, err := maddr.ValueForProtocol(ma.P_IPFS)
	require.NoError(err)

	peerid, err := peer.IDB58Decode(pid)
	require.NoError(err)

	// Decapsulate the /ipfs/<peerID> part from the target
	// /ip4/<a.b.c.d>/ipfs/<peer> becomes /ip4/<a.b.c.d>
	targetPeerAddr, _ := ma.NewMultiaddr(
		fmt.Sprintf("/ipfs/%s", peer.IDB58Encode(peerid)))
	targetAddr := maddr.Decapsulate(targetPeerAddr)

	t.Logf("opening stream, peerid: %s, targetAddr: %s", peerid, targetAddr)
	s.Host.Peerstore().AddAddr(peerid, targetAddr, pstore.PermanentAddrTTL)

	// make a new stream from host B to host A
	// it should be handled on host A by the handler we set above because
	// we use the same /echo/1.0.0 protocol
	stream, err := s.Host.NewStream(context.Background(), peerid, fcmetrics.HeartbeatProtocol)
	require.NoError(err)
	return stream
}

func TestNodeSimple(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	// generate a private key for the aggregators identity
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	require.NoError(err)

	// create a new aggregator and tell it to listen on port 0, this will cause the
	// kernel to assign the aggregator any available port
	a, err := New(ctx, 0, priv)
	require.NoError(err)

	// instead of calling the `Start` method, we will manually create an errorgroup
	// for the aggregator, and then start the input and filter routines, this
	// will allow us to read from the `Sink` channel (since the output routine is not running)
	// and make assertions on the output we receive in the loop below
	a.eg, a.ctx = errgroup.WithContext(ctx)
	require.NoError(a.startInput())
	require.NoError(a.startFilter())

	fc := newHeartbeatNode(t)
	stream := fc.StartAndConnect(t, a.FullAddress)
	enc := json.NewEncoder(stream)

	for i := 1; i < 10000; i++ {
		b := fc.Beat()
		require.NoError(enc.Encode(b))
		e := <-a.Sink
		require.Equal(uint64(i), e.Heartbeat.Height)
		require.Equal(fmt.Sprintf("tipset%d", i), e.Heartbeat.Tipset)
		require.Equal(fc.Host.ID().String(), e.FromPeer.String())
	}

}
