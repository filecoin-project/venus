package impl

import (
	"context"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/repo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ci "gx/ipfs/QmNiJiXwWE3kRhZrC5ej3kSjWHm337pYfhjLGSCDNKJP2s/go-libp2p-crypto"
	libp2p "gx/ipfs/QmVvV8JQmmqPCwXAaesWJPheUiEFQJ9HWRhWhuFuxVQxpR/go-libp2p"
	peer "gx/ipfs/QmcqU6QUDSXprb1518vYDGczrTJTyGwLG9eUa5iNX4xUtS/go-libp2p-peer"
)

func makeIdentityOption(t *testing.T) (libp2p.Option, ci.PrivKey) {
	// create a public private key pair
	sk, _, err := ci.GenerateKeyPair(ci.RSA, 1024)
	require.NoError(t, err)
	return libp2p.Identity(sk), sk
}

func makeSwarmAddressOption(t *testing.T, adder string) libp2p.Option {
	return libp2p.ListenAddrStrings(adder)
}

func TestIdOutput(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	ctx := context.Background()
	r := repo.NewInMemoryRepo()
	r.Config().Swarm.Address = "/ip4/0.0.0.0/tcp/0"

	err := node.Init(ctx, r, consensus.InitGenesis)
	assert.NoError(err)

	// define repo option
	var opts []node.ConfigOpt
	dsopt := func(c *node.Config) error {
		c.Repo = r
		return nil
	}

	// create our libp2p options
	idOpt, expectedPrivKey := makeIdentityOption(t)
	expectedSwarm := "/ip4/127.0.0.1/tcp/6888"

	opts = append(opts, dsopt, node.Libp2pOptions(
		makeSwarmAddressOption(t, expectedSwarm),
		idOpt,
	))

	// Create a new nodes with our opts
	nd, err := node.New(ctx, opts...)
	assert.NoError(err)
	api := New(nd)

	// call method being tested
	actualOut, err := api.ID().Details()
	assert.NoError(err)

	// create the expected peerID from our secrect key
	expectedPeerID, err := peer.IDFromPrivateKey(expectedPrivKey)
	assert.NoError(err)

	// We should have the expected peerID
	assert.EqualValues(expectedPeerID.Pretty(), actualOut.ID)

	// Should have expected swarmAddress
	assert.Contains(actualOut.Addresses[0], fmt.Sprintf("%s/ipfs/%s", expectedSwarm, expectedPeerID.Pretty()))
}
