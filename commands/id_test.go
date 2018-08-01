package commands

import (
	"context"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/repo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	libp2p "gx/ipfs/QmZ86eLPtXkQ1Dfa992Q8NpXArUoWWh3y728JDcWvzRrvC/go-libp2p"
	peer "gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"
	ci "gx/ipfs/Qme1knMqwt1hKZbc1BmQFmnm9f36nyQGwXxPGVpVJ9rMK5/go-libp2p-crypto"
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

	// call method being tested
	actualOut := idOutputFromNode(nd)

	// create the expected peerId from our secrect key
	expectedPeerID, err := peer.IDFromPrivateKey(expectedPrivKey)
	assert.NoError(err)

	// We should have the expected peerId
	assert.EqualValues(expectedPeerID.Pretty(), actualOut.ID)

	// Should have expected swarmAddress
	assert.Contains(actualOut.Addresses[0], fmt.Sprintf("%s/ipfs/%s", expectedSwarm, expectedPeerID.Pretty()))
}
