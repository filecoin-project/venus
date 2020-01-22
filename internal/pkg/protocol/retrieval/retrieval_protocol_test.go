package retrieval_test

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	"github.com/filecoin-project/go-filecoin/internal/pkg/protocol/retrieval"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

// NOTE: The test TestRetrievalProtocolHappyPath has been deleted due to flakiness.
// Coverage of this feature has been relegated to the functional-tests/retrieval script.
// See https://github.com/filecoin-project/go-filecoin/pull/1643

func TestRetrievalProtocolPieceNotFound(t *testing.T) {
	t.Skip("Skip pending retrieval market shared component")
	tf.UnitTest(t)

	ctx := context.Background()

	minerNode, _, minerAddr, _ := configureMinerAndClient(t)

	require.NoError(t, minerNode.StartMining(ctx))
	defer minerNode.StopMining(ctx)

	someRandomCid := types.NewCidForTestGetter()()

	minerPID, err := minerNode.PorcelainAPI.MinerGetPeerID(ctx, minerAddr)
	require.NoError(t, err)

	_, err = retrievePieceBytes(ctx, minerNode.RetrievalProtocol.RetrievalAPI, someRandomCid, minerPID, minerAddr)
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
	builder1 := test.NewNodeBuilder(t)
	builder1.WithInitOpt(node.PeerKeyOpt(node.PeerKeys[0]))
	builder1.WithGenesisInit(seed.GenesisInitFunc)
	builder2 := test.NewNodeBuilder(t)
	builder2.WithGenesisInit(seed.GenesisInitFunc)

	// make two nodes, one of which is the minerNode (and gets the miner peer key)
	minerNode = builder1.Build(ctx)
	clientNode = builder2.Build(ctx)

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
