package metrics_test

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	net "github.com/libp2p/go-libp2p-core/network"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/metrics"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

var testCid cid.Cid

func init() {
	c, err := cid.Decode("Qmd52WKRSwrBK5gUaJKawryZQ5by6UbNB8KVW2Zy6JtbyW")
	if err != nil {
		panic(err)
	}
	testCid = c
}

type endpoint struct {
	Host    host.Host
	Address string
}

func newEndpoint(t *testing.T, port int) endpoint {
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	opts := []libp2p.Option{
		libp2p.DisableRelay(),
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port)),
		libp2p.Identity(priv),
	}

	basicHost, err := libp2p.New(context.Background(), opts...)
	if err != nil {
		t.Fatal(err)
	}

	// Build host multiaddress
	hostAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", basicHost.ID().Pretty()))

	// Now we can build a full multiaddress to reach this host
	// by encapsulating both addresses:
	addr := basicHost.Addrs()[0]
	fullAddr := addr.Encapsulate(hostAddr)

	return endpoint{
		Host:    basicHost,
		Address: fullAddr.String(),
	}
}

func TestHeartbeatConnectSuccess(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	aggregator := newEndpoint(t, 0)
	filecoin := newEndpoint(t, 0)
	aggregator.Host.SetStreamHandler(metrics.HeartbeatProtocol, func(c net.Stream) {
	})

	hbs := metrics.NewHeartbeatService(
		filecoin.Host,
		testCid,
		&config.HeartbeatConfig{
			BeatTarget:      aggregator.Address,
			BeatPeriod:      "3s",
			ReconnectPeriod: "10s",
			Nickname:        "BobHoblaw",
		},
		func() (types.TipSet, error) {
			tipSet := chain.NewBuilder(t, address.Undef).NewGenesis()
			return tipSet, nil
		},
	)

	assert.Equal(t, 1, len(aggregator.Host.Peerstore().Peers()))
	assert.Contains(t, aggregator.Host.Peerstore().Peers(), aggregator.Host.ID())
	assert.NoError(t, hbs.Connect(ctx))
	assert.Equal(t, 2, len(aggregator.Host.Peerstore().Peers()))
	assert.Contains(t, aggregator.Host.Peerstore().Peers(), aggregator.Host.ID())
	assert.Contains(t, aggregator.Host.Peerstore().Peers(), filecoin.Host.ID())
}

func TestHeartbeatConnectFailure(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	filecoin := newEndpoint(t, 60001)

	hbs := metrics.NewHeartbeatService(
		filecoin.Host,
		testCid,
		&config.HeartbeatConfig{
			BeatTarget:      "",
			BeatPeriod:      "3s",
			ReconnectPeriod: "10s",
			Nickname:        "BobHoblaw",
		},
		func() (types.TipSet, error) {
			tipSet := chain.NewBuilder(t, address.Undef).NewGenesis()
			return tipSet, nil
		},
	)
	assert.Error(t, hbs.Connect(ctx))
}

func TestHeartbeatRunSuccess(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	// we will use this to stop the run method after making assertions
	runCtx, cancel := context.WithCancel(ctx)

	// port 0 to avoid conflicts
	aggregator := newEndpoint(t, 0)
	filecoin := newEndpoint(t, 0)

	// create a tipset, we will assert on it in the SetStreamHandler method
	expHeight := types.Uint64(444)
	expTs := mustMakeTipset(t, expHeight)

	addr, err := address.NewActorAddress([]byte("miner address"))
	require.NoError(t, err)

	// The handle method will run the assertions for the test
	aggregator.Host.SetStreamHandler(metrics.HeartbeatProtocol, func(s net.Stream) {
		defer func() {
			require.NoError(t, s.Close())
		}()

		dec := json.NewDecoder(s)
		var hb metrics.Heartbeat
		require.NoError(t, dec.Decode(&hb))

		assert.Equal(t, expTs.String(), hb.Head)
		assert.Equal(t, uint64(444), hb.Height)
		assert.Equal(t, "BobHoblaw", hb.Nickname)
		assert.Equal(t, addr, hb.MinerAddress)
		cancel()
	})

	hbs := metrics.NewHeartbeatService(
		filecoin.Host,
		testCid,
		&config.HeartbeatConfig{
			BeatTarget:      aggregator.Address,
			BeatPeriod:      "1s",
			ReconnectPeriod: "1s",
			Nickname:        "BobHoblaw",
		},
		func() (types.TipSet, error) {
			return expTs, nil
		},
		metrics.WithMinerAddressGetter(func() address.Address {
			return addr
		}),
	)

	require.NoError(t, hbs.Connect(ctx))

	assert.NoError(t, hbs.Run(runCtx))
	assert.Error(t, runCtx.Err(), context.Canceled.Error())
}

func mustMakeTipset(t *testing.T, height types.Uint64) types.TipSet {
	ts, err := types.NewTipSet(&types.Block{
		Miner:           address.NewForTestGetter()(),
		Ticket:          nil,
		Parents:         types.TipSetKey{},
		ParentWeight:    0,
		Height:          height,
		Nonce:           0,
		MessageReceipts: types.EmptyMessagesCID,
		Messages:        types.EmptyReceiptsCID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return ts
}
