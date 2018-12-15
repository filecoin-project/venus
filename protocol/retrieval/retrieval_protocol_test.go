package retrieval_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"testing"
	"time"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmVDTbzzTwnuBwNbJdhW3u7LoBQp46bezm9yp4z1RoEepM/go-blockservice"
	"gx/ipfs/QmdURv6Sbob8TVW2tFFve9vcEWrSUgwPqeqnXyvYhLrkyd/go-merkledag"

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

	minerNode, clientNode, minerAddr, _ := configureMinerAndClient(t)

	require.NoError(minerNode.StartMining(ctx))
	defer minerNode.StopMining(ctx)

	someRandomCid := types.NewCidForTestGetter()()

	_, err := retrievePieceBytes(ctx, impl.New(clientNode).RetrievalClient(), someRandomCid, minerAddr)
	require.Error(err)
}

func TestRetrievalProtocolHappyPath(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	ctx := context.Background()

	minerNode, clientNode, minerAddr, minerOwnerAddr := configureMinerAndClient(t)

	// start mining
	require.NoError(minerNode.StartMining(ctx))
	defer minerNode.StopMining(ctx)

	testSectorSize, err := minerNode.SectorBuilder().GetMaxUserBytesPerStagedSector()
	require.NoError(err)

	// pretend like we've run through the storage protocol and saved user's
	// data to the miner's block store and sector builder
	pieceA, bytesA := createRandomPieceInfo(t, minerNode.BlockService(), testSectorSize/2)
	pieceB, bytesB := createRandomPieceInfo(t, minerNode.BlockService(), testSectorSize-(testSectorSize/2))

	_, err = minerNode.SectorBuilder().AddPiece(ctx, pieceA) // blocks until all piece-bytes written to sector
	require.NoError(err)
	_, err = minerNode.SectorBuilder().AddPiece(ctx, pieceB) // triggers seal
	require.NoError(err)

	// wait for commitSector to make it into the chain
	cancelA := make(chan struct{})
	cancelB := make(chan struct{})
	errCh := make(chan error)
	defer close(cancelA)
	defer close(cancelB)
	defer close(errCh)

	select {
	case <-firstMatchingMsgInChain(ctx, t, minerNode.ChainReader, "commitSector", minerOwnerAddr, cancelA, errCh):
	case err = <-errCh:
		require.NoError(err)
	case <-time.After(120 * time.Second):
		cancelA <- struct{}{}
		t.Fatalf("timed out waiting for commitSector message (for sector of size=%d, from miner owner=%s) to appear in **miner** node's chain", testSectorSize, minerOwnerAddr)
	}

	select {
	case <-firstMatchingMsgInChain(ctx, t, clientNode.ChainReader, "commitSector", minerOwnerAddr, cancelB, errCh):
	case err = <-errCh:
		require.NoError(err)
	case <-time.After(120 * time.Second):
		cancelB <- struct{}{}
		t.Fatalf("timed out waiting for commitSector message (for sector of size=%d, from miner owner=%s) to appear in **client** node's chain", testSectorSize, minerOwnerAddr)
	}

	// retrieve piece by CID and compare bytes with what we sent to miner
	retrievedBytesA, err := retrievePieceBytes(ctx, impl.New(clientNode).RetrievalClient(), pieceA.Ref, minerAddr)
	require.NoError(err)
	require.True(bytes.Equal(bytesA, retrievedBytesA))

	// retrieve/compare the second piece for good measure
	retrievedBytesB, err := retrievePieceBytes(ctx, impl.New(clientNode).RetrievalClient(), pieceB.Ref, minerAddr)
	require.NoError(err)
	require.True(bytes.Equal(bytesB, retrievedBytesB))

	// sanity check
	require.True(len(retrievedBytesA) > 0)
}

func retrievePieceBytes(ctx context.Context, retrievalClient api.RetrievalClient, data cid.Cid, addr address.Address) ([]byte, error) {
	r, err := retrievalClient.RetrievePiece(ctx, data, addr)
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
// Note: this function should probably be API porcelain.
func firstMatchingMsgInChain(ctx context.Context, t *testing.T, chainManager chain.ReadStore, msgMethod string, msgSenderAddr address.Address, cancelCh <-chan struct{}, errCh chan<- error) <-chan *types.Block {
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
				case event, more := <-history:
					if !more {
						break Loop
					}

					ts, ok := event.(consensus.TipSet)
					if !ok {
						errCh <- fmt.Errorf("expected a TipSet, got a %T with value %v", event, event)
						return
					}

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
	err := blockService.AddBlock(node)
	require.NoError(t, err)

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
