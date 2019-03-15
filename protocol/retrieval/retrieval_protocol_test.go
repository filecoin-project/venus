package retrieval_test

import (
	"context"
	"io/ioutil"
	"testing"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/protocol/retrieval"
	"github.com/filecoin-project/go-filecoin/types"
)

// NOTE: The test TestRetrievalProtocolHappyPath has been deleted due to flakiness.
// Coverage of this feature has been relegated to the functional-tests/retrieval script.
// See https://github.com/filecoin-project/go-filecoin/pull/1643

func TestRetrievalProtocolPieceNotFound(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	ctx := context.Background()

	minerNode, _, minerAddr, _ := configureMinerAndClient(t)

	require.NoError(minerNode.StartMining(ctx))
	defer minerNode.StopMining(ctx)

	someRandomCid := types.NewCidForTestGetter()()

	_, err := retrievePieceBytes(ctx, minerNode.RetrievalAPI, someRandomCid, minerAddr)
	require.Error(err)
}

func retrievePieceBytes(ctx context.Context, retrievalAPI *retrieval.API, data cid.Cid, addr address.Address) ([]byte, error) {
	r, err := retrievalAPI.RetrievePiece(ctx, data, addr)
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
