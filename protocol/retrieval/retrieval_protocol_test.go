package retrieval_test

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/protocol/retrieval"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

// NOTE: The test TestRetrievalProtocolHappyPath has been deleted due to flakiness.
// Coverage of this feature has been relegated to the functional-tests/retrieval script.
// See https://github.com/filecoin-project/go-filecoin/pull/1643

func TestRetrievalProtocolPieceNotFound(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	minerNode, _, minerAddr, _ := configureMinerAndClient(t)

	require.NoError(t, minerNode.StartMining(ctx))
	defer minerNode.StopMining(ctx)

	someRandomCid := types.NewCidForTestGetter()()

	minerPID, err := minerNode.PorcelainAPI.MinerGetPeerID(ctx, minerAddr)
	require.NoError(t, err)

	_, err = retrievePieceBytes(ctx, minerNode.RetrievalAPI, someRandomCid, minerPID, minerAddr)
	require.Error(t, err)
}

func retrievePieceBytes(ctx context.Context, retrievalAPI *retrieval.API, data cid.Cid, minerPID peer.ID, addr address.Address) ([]byte, error) {
	r, err := retrievalAPI.RetrievePiece(ctx, data, minerPID, addr)
	if err != nil {
		return nil, err
	}

	slice, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return slice, nil
}

func configureMinerAndClient(t *testing.T) (minerNode *node.Node, clientNode *node.Node, minerAddr address.Address, minerOwnerAddr address.Address) {
	ctx := context.Background()

	seed := node.MakeChainSeed(t, node.TestGenCfg)

	// make two nodes, one of which is the minerNode (and gets the miner peer key)
	minerNode = node.MakeNodeWithChainSeed(t, seed, []node.ConfigOpt{}, node.PeerKeyOpt(node.PeerKeys[0]), node.AutoSealIntervalSecondsOpt(0))
	clientNode = node.MakeNodeWithChainSeed(t, seed, []node.ConfigOpt{})

	// give the minerNode node a key and the miner associated with that key
	seed.GiveKey(t, minerNode, 0)
	minerAddr, minerOwnerAddr = seed.GiveMiner(t, minerNode, 0)

	// give the clientNode node a private key, too
	seed.GiveKey(t, clientNode, 1)

	// start 'em up
	require.NoError(t, minerNode.Start(ctx))
	require.NoError(t, clientNode.Start(ctx))

	// make sure they're swarmed together (for block propagation)
	node.ConnectNodes(t, minerNode, clientNode)

	return
}
