package impl

import (
	"context"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/repo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ci "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	peer "gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	libp2p "gx/ipfs/QmVM6VuGaWcAaYjxG2om6XxMmpP3Rt9rw4nbMXVNYAPLhS/go-libp2p"
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

	err := node.Init(ctx, r, core.InitGenesis)
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
