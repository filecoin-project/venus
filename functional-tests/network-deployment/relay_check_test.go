package networkdeployment_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/fixtures"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/fastesting"

	"github.com/ipfs/go-cid"
	pr "github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multiaddr-dns"
)

// TestRelayCheck is a two part test
// 1) Check that the relay peers are advertising their addresses under
//    the correct dht key
// 2) Check that a node behind a NAT aquires a circuit relay address from
//    one of the relay peers
func TestRelayCheck(t *testing.T) {
	network := tf.DeploymentTest(t)

	ctx := context.Background()
	ctx, env := fastesting.NewDeploymentEnvironment(ctx, t, network, fast.FilecoinOpts{})
	defer func() {
		err := env.Teardown(ctx)
		require.NoError(t, err)
	}()

	client := env.RequireNewNodeStarted()

	// In this test we query the dht looking for providers of the relay key
	// and verify that all of the expected providers show up at some point.
	t.Run("Check for relay providers", func(t *testing.T) {
		dhtKey, err := cid.Decode("zb2rhZ6FpTqFZyiAtpQFRKmybPMjq5A7oPHfmD5WeBko5kRAo")
		require.NoError(t, err)

		maddrChan := make(chan multiaddr.Multiaddr, 16)
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		eventChan, err := scanDhtProviders(ctx, t, client, dhtKey)
		require.NoError(t, err)

		go func() {
			defer close(maddrChan)
			for event := range eventChan {
				// There is only a single PeerInfo in the response see command `findprovs`.
				pinfo := event.Responses[0]

				// For some reason if a peer responses to a query, it will not include its
				// own addresses. This might be a bug. However, all it means is that we need
				// to take at least two arounds from two different peers, which is largely
				// just luck of the draw.
				if len(pinfo.Addrs) == 0 {
					t.Logf("No addresses returned for peer %s", pinfo.ID)
					continue
				}

				t.Logf("Found record for peer %s", pinfo.ID)

				// Converts the pinfo into a set of addresses
				maddrs, err := pr.AddrInfoToP2pAddrs(pinfo)
				if err != nil {
					t.Logf("Failed to get maddrs")
					continue
				}

				for _, maddr := range maddrs {
					maddrChan <- maddr
				}
			}
		}()

		relayPeersAddrs := networkRelayPeers(network)
		require.NotEmpty(t, relayPeersAddrs)

		// To verify that all of the relay peers are advertising correctly we need to
		// see one of their addresses come through when we query the relay provider key.
		// Below we construct a peer.ID mapping to a PeerInfo that contains addresses we
		// expect to see.
		// The address we expect to see is either the dns4 multiaddr from relayPeersAddrs,
		// or the resolved ip4 address.
		relayPeers, err := createResolvedPeerInfoMap(ctx, relayPeersAddrs)
		require.NoError(t, err)

		for maddr := range maddrChan {
			pinfo, err := pr.AddrInfoFromP2pAddr(maddr)
			require.NoError(t, err)

			if _, ok := relayPeers[pinfo.ID]; !ok {
				continue
			}

			// pinfo will have only a single address as it comes from a single multiaddr
			require.NotEmpty(t, pinfo.Addrs)
			addr := pinfo.Addrs[0]

			t.Logf("Looking at addr %s", addr)
			for _, a := range relayPeers[pinfo.ID].Addrs {
				if addr.Equal(a) {
					t.Logf("Found addr for peer %s", pinfo.ID)
					delete(relayPeers, pinfo.ID)
				}
			}

			if len(relayPeers) == 0 {
				cancel()
			}

			for peerID := range relayPeers {
				t.Logf("Still waiting for %s", peerID)
			}
		}
	})

	// In this test we want to verify that we retrieve a circuit relay address
	// from one of our expected relay providers.
	t.Run("Has circuit address", func(t *testing.T) {
		details, err := client.ID(ctx)
		require.NoError(t, err)

		maddrChan := make(chan multiaddr.Multiaddr, 16)
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		go func() {
			defer close(maddrChan)
			for {
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
					details, err := client.ID(ctx)
					assert.NoError(t, err)

					for _, maddr := range details.Addresses {
						maddrChan <- maddr
					}
				}
			}
		}()

		relayPeersAddrs := networkRelayPeers(network)
		require.NotEmpty(t, relayPeersAddrs)

		relayPeers, err := createResolvedPeerInfoMap(ctx, relayPeersAddrs)
		require.NoError(t, err)

		// To verify that we have a circuit address from one of our relays we need to
		// strip off the circuit component, and compare the address to the known addresses
		// of our relays

		protop2p := multiaddr.ProtocolWithCode(multiaddr.P_P2P)
		protocircuit := multiaddr.ProtocolWithCode(multiaddr.P_CIRCUIT)

		// /ipfs/<ID>
		peercomp, err := multiaddr.NewComponent(protop2p.Name, details.ID.String())
		require.NoError(t, err)

		// /p2p-circuit
		relaycomp, err := multiaddr.NewComponent(protocircuit.Name, "")
		require.NoError(t, err)

		// /p2p-circuit/ipfs/<ID>
		relaypeer := relaycomp.Encapsulate(peercomp)

		for maddr := range maddrChan {
			if _, err := maddr.ValueForProtocol(multiaddr.P_CIRCUIT); err != nil {
				continue
			}

			t.Logf("Found circuit addr %s", maddr)

			relayaddr := maddr.Decapsulate(relaypeer)
			pinfo, err := pr.AddrInfoFromP2pAddr(relayaddr)
			require.NoError(t, err)

			if _, ok := relayPeers[pinfo.ID]; !ok {
				continue
			}

			// pinfo will have only a single address as it comes from a single multiaddr
			require.NotEmpty(t, pinfo.Addrs)
			addr := pinfo.Addrs[0]

			if _, ok := relayPeers[pinfo.ID]; !ok {
				t.Logf("Found circuit address %s from unexpected peer %s", maddr, pinfo.ID)
				continue
			}

			for _, a := range relayPeers[pinfo.ID].Addrs {
				if addr.Equal(a) {
					t.Logf("Addr relays through %s", pinfo.ID)
					cancel()
					break
				}
			}
		}
	})
}

func createResolvedPeerInfoMap(ctx context.Context, addrs []string) (map[pr.ID]*pr.AddrInfo, error) {
	protop2p := multiaddr.ProtocolWithCode(multiaddr.P_P2P)

	relayPeers := make(map[pr.ID]*pr.AddrInfo)
	for _, addr := range addrs {
		maddr, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}

		pinfo, err := pr.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			return nil, err
		}

		// PeerInfo stores just the transport of the multiaddr and removes the peer
		// component from the end. However, when we resolve the dns4 address to ip4
		// we get back the full address, so we want to strip the peer component
		// for consistently

		// This is the /ipfs/<peer-id> component
		peercomp, err := multiaddr.NewComponent(protop2p.Name, pinfo.ID.String())
		if err != nil {
			return nil, err
		}

		rmaddrs, err := madns.Resolve(ctx, maddr)
		if err != nil {
			return nil, err
		}

		for _, maddr := range rmaddrs {
			pinfo.Addrs = append(pinfo.Addrs, maddr.Decapsulate(peercomp))
		}

		relayPeers[pinfo.ID] = pinfo
	}

	return relayPeers, nil
}

// scanDhtProviders runs a `findprovs` at least every 5 seconds and reads through all
// of the events looking for `notif.Provider` events. These events contain PeerInfo
// which we convert into a slice of multiaddrs and publish on our maddrChan.
func scanDhtProviders(ctx context.Context, t *testing.T, node *fast.Filecoin, dhtKey cid.Cid) (<-chan routing.QueryEvent, error) {
	eventChan := make(chan routing.QueryEvent, 16)

	go func() {
		defer close(eventChan)
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				t.Logf("Finding Providers for %s", dhtKey)
				decoder, err := node.DHTFindProvs(ctx, dhtKey)
				if err != nil {
					t.Logf("Failed to run `findprovs`: %s", err)
					continue
				}

				// Read all of the events from `findprovs`
				for {
					var event routing.QueryEvent
					if err := decoder.Decode(&event); err != nil {
						if err == io.EOF {
							break
						}

						t.Logf("Decode failed %s", err)
						continue
					}

					if event.Type == routing.Provider {
						if len(event.Responses) == 0 {
							t.Logf("No responses for provider event")
							continue
						}

						eventChan <- event
					}
				}
			}
		}
	}()

	return eventChan, nil
}

// returns the list of peer address for the network bootstrap peers
func networkBootstrapPeers(network string) []string {
	// Currently all bootstrap addresses are relay peers

	switch network {
	case "nightly":
		return fixtures.DevnetNightlyBootstrapAddrs
	case "staging":
		return fixtures.DevnetStagingBootstrapAddrs
	case "user":
		return fixtures.DevnetUserBootstrapAddrs
	}

	return []string{}
}

// returns the list of peer address for network relay peers
func networkRelayPeers(network string) []string {
	return networkBootstrapPeers(network)
}
