package impl

import (
	"context"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/repo"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"

	ma "gx/ipfs/QmNTCey11oxhb1AxDnQBRHtdhap6Ctud872NjAYPYYXPuc/go-multiaddr"
	ci "gx/ipfs/QmTW4SdgBWq9GjsBsHeUx8WuGxzhgzAf88UMH2w62PC8yK/go-libp2p-crypto"
	peer "gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"
	libp2p "gx/ipfs/QmcNGX5RaxPPCYwa6yGXM1EcUbrreTTinixLcYGmMwf1sx/go-libp2p"
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
	assert.EqualValues(expectedPeerID, actualOut.ID)

	// Should have expected swarmAddress
	expectedAddr, err := ma.NewMultiaddr(fmt.Sprintf("%s/ipfs/%s", expectedSwarm, expectedPeerID.Pretty()))
	assert.NoError(err)
	assert.Equal(actualOut.Addresses[0], expectedAddr)
}

func TestJSONRoundTrip(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)

	id, err := peer.IDB58Decode("QmV6guChs1SU6W9838XrWSSW2WeiDt4WjzB7Dz5HFFeLkG")
	require.NoError(err)

	addr, err := ma.NewMultiaddr("/tcp/80")
	require.NoError(err)
	idd := api.IDDetails{
		Addresses:       []ma.Multiaddr{addr},
		ID:              id,
		AgentVersion:    "1",
		ProtocolVersion: "1",
		PublicKey:       []byte{1, 2, 3, 4, 5, 6, 7, 8},
	}
	json, err := idd.MarshalJSON()
	assert.NoError(err)

	decoded := api.IDDetails{}
	err = decoded.UnmarshalJSON(json)
	assert.NoError(err)

	assert.Equal(idd, decoded)
}
