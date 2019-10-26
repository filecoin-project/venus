package networkdeployment_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/fastesting"

	pr "github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
)

// TestBootstrap verifies information about the bootstrap peers
func TestBootstrap(t *testing.T) {
	network := tf.DeploymentTest(t)

	ctx := context.Background()
	ctx, env := fastesting.NewDeploymentEnvironment(ctx, t, network, fast.FilecoinOpts{})
	defer func() {
		err := env.Teardown(ctx)
		require.NoError(t, err)
	}()

	client := env.RequireNewNodeStarted()

	t.Run("Check that we are connected to bootstrap peers", func(t *testing.T) {
		maddrChan := make(chan multiaddr.Multiaddr, 16)
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		go func() {
			defer close(maddrChan)
			protop2p := multiaddr.ProtocolWithCode(multiaddr.P_P2P)
			for {
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
					peers, err := client.SwarmPeers(ctx)
					assert.NoError(t, err)

					for _, peer := range peers {
						transport, err := multiaddr.NewMultiaddr(peer.Addr)
						require.NoError(t, err)

						// /ipfs/<ID>
						peercomp, err := multiaddr.NewComponent(protop2p.Name, peer.Peer)
						require.NoError(t, err)

						fullmaddr := transport.Encapsulate(peercomp)
						maddrChan <- fullmaddr
					}
				}
			}
		}()

		bootstrapAddrs := networkBootstrapPeers(network)
		require.NotEmpty(t, bootstrapAddrs)

		bootstrapPeers, err := createResolvedPeerInfoMap(ctx, bootstrapAddrs)
		require.NoError(t, err)

		for maddr := range maddrChan {
			pinfo, err := pr.AddrInfoFromP2pAddr(maddr)
			require.NoError(t, err)

			if _, ok := bootstrapPeers[pinfo.ID]; !ok {
				continue
			}

			// pinfo will have only a single address as it comes from a single multiaddr
			require.NotEmpty(t, pinfo.Addrs)
			addr := pinfo.Addrs[0]

			t.Logf("Looking at addr %s", addr)
			for _, a := range bootstrapPeers[pinfo.ID].Addrs {
				if addr.Equal(a) {
					t.Logf("Found addr for peer %s", pinfo.ID)
					delete(bootstrapPeers, pinfo.ID)
				}
			}

			if len(bootstrapPeers) == 0 {
				cancel()
			}

			for peerID := range bootstrapPeers {
				t.Logf("Still waiting for %s", peerID)
			}
		}
	})
}
