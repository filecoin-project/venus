package metrics

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ma "gx/ipfs/QmNTCey11oxhb1AxDnQBRHtdhap6Ctud872NjAYPYYXPuc/go-multiaddr"
	"gx/ipfs/QmNiJiXwWE3kRhZrC5ej3kSjWHm337pYfhjLGSCDNKJP2s/go-libp2p-crypto"
	"gx/ipfs/QmZ7cBWUXkyWTMN4qH6NGoyMVs7JugyFChBNP4ZUp5rJHH/go-libp2p-net"
	"gx/ipfs/QmebEmt23jQxrwnqBkFL4qbpE8EnnQunpv5U32LS5ESus1/go-libp2p"
	"gx/ipfs/QmfRHxh8bt4jWLKRhNvR5fn7mFACrQBFLqV4wyoymEExKV/go-libp2p-host"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/fixtures"
	"github.com/filecoin-project/go-filecoin/types"
)

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
	assert := assert.New(t)
	ctx := context.Background()
	aggregator := newEndpoint(t, 0)
	filecoin := newEndpoint(t, 0)
	aggregator.Host.SetStreamHandler(HeartbeatProtocol, func(c net.Stream) {
	})

	hbs := NewHeartbeatService(
		filecoin.Host,
		&config.HeartbeatConfig{
			BeatTarget:      aggregator.Address,
			BeatPeriod:      "3s",
			ReconnectPeriod: "10s",
			Nickname:        "BobHoblaw",
		},
		func() types.TipSet {
			return types.TipSet{
				"tippywippy": nil,
			}
		},
	)

	assert.Equal(1, len(aggregator.Host.Peerstore().Peers()))
	assert.Contains(aggregator.Host.Peerstore().Peers(), aggregator.Host.ID())
	assert.NoError(hbs.Connect(ctx))
	assert.Equal(2, len(aggregator.Host.Peerstore().Peers()))
	assert.Contains(aggregator.Host.Peerstore().Peers(), aggregator.Host.ID())
	assert.Contains(aggregator.Host.Peerstore().Peers(), filecoin.Host.ID())
}

func TestHeartbeatConnectFailure(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	filecoin := newEndpoint(t, 60001)

	hbs := NewHeartbeatService(
		filecoin.Host,
		&config.HeartbeatConfig{
			BeatTarget:      "",
			BeatPeriod:      "3s",
			ReconnectPeriod: "10s",
			Nickname:        "BobHoblaw",
		},
		func() types.TipSet {
			return types.TipSet{
				"tippywippy": nil,
			}
		},
	)
	assert.Error(hbs.Connect(ctx))
}

func TestHeartbeatRunSuccess(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ctx := context.Background()
	// we will use this to stop the run method after making assertions
	runCtx, cancel := context.WithCancel(ctx)

	// port 0 to avoid conflicts
	aggregator := newEndpoint(t, 0)
	filecoin := newEndpoint(t, 0)

	// create a tipset, we will assert on it in the SetStreamHandler method
	expHeight := types.Uint64(444)
	expTs := mustMakeTipset(t, expHeight)

	addr, err := address.NewFromString(fixtures.TestAddresses[0])
	require.NoError(err)

	// The handle method will run the assertions for the test
	aggregator.Host.SetStreamHandler(HeartbeatProtocol, func(s net.Stream) {
		defer s.Close()

		dec := json.NewDecoder(s)
		var hb Heartbeat
		require.NoError(dec.Decode(&hb))

		assert.Equal(expTs.String(), hb.Head)
		assert.Equal(uint64(444), hb.Height)
		assert.Equal("BobHoblaw", hb.Nickname)
		assert.Equal(addr, hb.MinerAddress)
		cancel()
	})

	hbs := NewHeartbeatService(
		filecoin.Host,
		&config.HeartbeatConfig{
			BeatTarget:      aggregator.Address,
			BeatPeriod:      "1s",
			ReconnectPeriod: "1s",
			Nickname:        "BobHoblaw",
		},
		func() types.TipSet {
			return expTs
		},
		WithMinerAddressGetter(func() address.Address {
			return addr
		}),
	)

	require.NoError(hbs.Connect(ctx))

	assert.NoError(hbs.Run(runCtx))
	assert.Error(runCtx.Err(), context.Canceled.Error())

}

func mustMakeTipset(t *testing.T, height types.Uint64) types.TipSet {
	ts, err := types.NewTipSet(&types.Block{
		Miner:           address.NewForTestGetter()(),
		Ticket:          nil,
		Parents:         types.SortedCidSet{},
		ParentWeight:    0,
		Height:          types.Uint64(height),
		Nonce:           0,
		Messages:        nil,
		MessageReceipts: nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	return ts
}
