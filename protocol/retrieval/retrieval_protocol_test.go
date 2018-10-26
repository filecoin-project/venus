package retrieval_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"io/ioutil"
	"testing"
	"time"

	"gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	"gx/ipfs/QmTfTKeBhTLjSjxXQsjkF2b1DfZmYEMnknGE2y2gX57C6v/go-blockservice"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	"gx/ipfs/QmeLG6jF1xvEmHca5Vy4q4EdQWp8Xq9S6EPyZrN9wvSRLC/go-merkledag"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/api/impl"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/types"

	"github.com/stretchr/testify/require"
)

func TestRetrievalProtocolPieceNotFound(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	ctx := context.Background()

	minerNode, clientNode, _, _ := configureMinerAndClient(t)

	require.NoError(minerNode.StartMining(ctx))
	defer minerNode.StopMining(ctx)

	someRandomCid := types.NewCidForTestGetter()()

	_, err := retrievePieceBytes(ctx, impl.New(clientNode).RetrievalClient(), minerNode.Host().ID(), someRandomCid)
	require.Error(err)
}

func TestRetrievalProtocolHappyPath(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	ctx := context.Background()

	minerNode, clientNode, _, minerOwnerAddr := configureMinerAndClient(t)

	// start mining
	require.NoError(minerNode.StartMining(ctx))
	defer minerNode.StopMining(ctx)

	response, err := minerNode.SectorStore.GetMaxUnsealedBytesPerSector()
	require.NoError(err)
	testSectorSize := uint64(response.NumBytes)

	// pretend like we've run through the storage protocol and saved user's
	// data to the miner's block store and sector builder
	pieceA, bytesA := createRandomPieceInfo(t, minerNode.BlockService(), testSectorSize/2)
	pieceB, bytesB := createRandomPieceInfo(t, minerNode.BlockService(), testSectorSize-(testSectorSize/2))

	minerNode.SectorBuilder().AddPiece(ctx, pieceA) // blocks until all piece-bytes written to sector
	minerNode.SectorBuilder().AddPiece(ctx, pieceB) // triggers seal

	// wait for commitSector to make it into the chain
	cancelA := make(chan struct{})
	cancelB := make(chan struct{})
	defer close(cancelA)
	defer close(cancelB)

	select {
	case <-firstMatchingMsgInChain(ctx, t, minerNode.ChainReader, "commitSector", minerOwnerAddr, cancelA):
	case <-time.After(120 * time.Second):
		cancelA <- struct{}{}
		t.Fatalf("timed out waiting for commitSector message (for sector of size=%d, from miner owner=%s) to appear in miner node's chain", testSectorSize, minerOwnerAddr)
	}

	select {
	case <-firstMatchingMsgInChain(ctx, t, clientNode.ChainReader, "commitSector", minerOwnerAddr, cancelB):
	case <-time.After(120 * time.Second):
		cancelB <- struct{}{}
		t.Fatalf("timed out waiting for commitSector message (for sector of size=%d, from miner owner=%s) to appear in client node's chain", testSectorSize, minerOwnerAddr)
	}

	// retrieve piece by CID and compare bytes with what we sent to miner
	retrievedBytesA, err := retrievePieceBytes(ctx, impl.New(clientNode).RetrievalClient(), minerNode.Host().ID(), pieceA.Ref)
	require.NoError(err)
	require.True(bytes.Equal(bytesA, retrievedBytesA))

	// retrieve/compare the second piece for good measure
	retrievedBytesB, err := retrievePieceBytes(ctx, impl.New(clientNode).RetrievalClient(), minerNode.Host().ID(), pieceB.Ref)
	require.NoError(err)
	require.True(bytes.Equal(bytesB, retrievedBytesB))

	// sanity check
	require.True(len(retrievedBytesA) > 0)
}

func retrievePieceBytes(ctx context.Context, retrievalClient api.RetrievalClient, minerPeerID peer.ID, pieceCid *cid.Cid) ([]byte, error) {
	r, err := retrievalClient.RetrievePiece(ctx, minerPeerID, pieceCid)
	if err != nil {
		return nil, err
	}

	slice, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return slice, nil
}

// firstMatchingMsgInChain queries the blockchain history in a loop for a block
// with matching properties, publishing the first match to the returned channel.
func firstMatchingMsgInChain(ctx context.Context, t *testing.T, chainManager chain.ReadStore, msgMethod string, msgSenderAddr address.Address, cancelCh <-chan struct{}) <-chan *types.Block {
	out := make(chan *types.Block)

	go func() {
		defer close(out)

		for {
			history := chainManager.BlockHistory(ctx)

		Loop:
			for {
				select {
				case <-cancelCh:
					return
				case event, ok := <-history:
					if !ok {
						break Loop
					}

					ts, ok := event.(consensus.TipSet)
					require.True(t, ok, "expected a TipSet")

					for _, block := range ts.ToSlice() {
						for _, message := range block.Messages {
							if message.Method == msgMethod && message.From == msgSenderAddr {
								out <- block
								return
							}
						}
					}
				}
			}

			time.Sleep(time.Millisecond * 100)
		}
	}()

	return out
}

func createRandomPieceInfo(t *testing.T, blockService blockservice.BlockService, n uint64) (*sectorbuilder.PieceInfo, []byte) {
	randomBytes := createRandomBytes(t, n)
	node := merkledag.NewRawNode(randomBytes)
	blockService.AddBlock(node)

	size, err := node.Size()
	require.NoError(t, err)

	return &sectorbuilder.PieceInfo{
		Ref:  node.Cid(),
		Size: size,
	}, randomBytes
}

func createRandomBytes(t *testing.T, n uint64) []byte {
	slice := make([]byte, n)

	_, err := io.ReadFull(rand.Reader, slice)
	require.NoError(t, err)

	return slice
}

func configureMinerAndClient(t *testing.T) (minerNode *node.Node, clientNode *node.Node, minerAddr address.Address, minerOwnerAddr address.Address) {
	ctx := context.Background()

	seed := node.MakeChainSeed(t, node.TestGenCfg)

	// make two nodes, one of which is the minerNode (and gets the miner peer key)
	minerNode = node.NodeWithChainSeed(t, seed, node.PeerKeyOpt(node.PeerKeys[0]), node.AutoSealIntervalSecondsOpt(0))
	clientNode = node.NodeWithChainSeed(t, seed)

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
