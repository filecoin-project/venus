package node

import (
	"context"
	"github.com/filecoin-project/go-filecoin/address"
	"testing"
	"time"

	"gx/ipfs/QmQAGG1zxfePqj2t7bLxyN8AFccZ889DDR9Gn8kVLDrGZo/go-libp2p-peerstore"

	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/protocol/storage"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func connect(t *testing.T, nd1, nd2 *Node) {
	t.Helper()
	pinfo := peerstore.PeerInfo{
		ID:    nd2.Host().ID(),
		Addrs: nd2.Host().Addrs(),
	}

	if err := nd1.Host().Connect(context.Background(), pinfo); err != nil {
		t.Fatal(err)
	}
}

func stopNodes(nds []*Node) {
	for _, nd := range nds {
		nd.Stop(context.Background())
	}
}

func startNodes(t *testing.T, nds []*Node) {
	t.Helper()
	for _, nd := range nds {
		if err := nd.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBlockPropTwoNodes(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	assert := assert.New(t)

	minerAddr, nodes := makeNodes(ctx, t, assert)
	startNodes(t, nodes)
	defer stopNodes(nodes)

	minerNode := nodes[0]
	clientNode := nodes[1]

	connect(t, minerNode, clientNode)

	baseTS := nodes[0].ChainReader.Head()
	require.NotNil(t, baseTS)
	proof := consensus.MakePoStProof()

	nextBlk := &types.Block{
		Miner:             minerAddr,
		Parents:           baseTS.ToSortedCidSet(),
		Height:            types.Uint64(1),
		ParentWeightNum:   types.Uint64(10),
		ParentWeightDenom: types.Uint64(1),
		StateRoot:         baseTS.ToSlice()[0].StateRoot,
		Proof:             proof,
		Ticket:            consensus.CreateTicket(proof, minerAddr),
	}

	// Wait for network connection notifications to propagate
	time.Sleep(time.Millisecond * 75)

	assert.NoError(nodes[0].AddNewBlock(ctx, nextBlk))

	equal := false
	for i := 0; i < 30; i++ {
		otherHead := nodes[1].ChainReader.Head()
		assert.NotNil(t, otherHead)
		equal = otherHead.ToSlice()[0].Cid().Equals(nextBlk.Cid())
		if equal {
			break
		}
		time.Sleep(time.Millisecond * 20)
	}

	assert.True(equal, "failed to sync chains")
}

func TestChainSync(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	assert := assert.New(t)

	minerAddr, nodes := makeNodes(ctx, t, assert)
	startNodes(t, nodes)
	defer stopNodes(nodes)

	baseTS := nodes[0].ChainReader.Head()
	nextBlk1 := consensus.NewValidTestBlockFromTipSet(baseTS, 1, minerAddr)
	nextBlk2 := consensus.NewValidTestBlockFromTipSet(baseTS, 2, minerAddr)
	nextBlk3 := consensus.NewValidTestBlockFromTipSet(baseTS, 3, minerAddr)

	assert.NoError(nodes[0].AddNewBlock(ctx, nextBlk1))
	assert.NoError(nodes[0].AddNewBlock(ctx, nextBlk2))
	assert.NoError(nodes[0].AddNewBlock(ctx, nextBlk3))

	connect(t, nodes[0], nodes[1])

	equal := false
	for i := 0; i < 30; i++ {
		otherHead := nodes[1].ChainReader.Head()
		assert.NotNil(t, otherHead)
		equal = otherHead.ToSlice()[0].Cid().Equals(nextBlk3.Cid())
		if equal {
			break
		}
		time.Sleep(time.Millisecond * 20)
	}

	assert.True(equal, "failed to sync chains")
}

func makeNodes(ctx context.Context, t *testing.T, assertions *assert.Assertions) (address.Address, []*Node) {
	seed := MakeChainSeed(t, TestGenCfg)
	minerNode := MakeNodeWithChainSeed(t, seed, PeerKeyOpt(PeerKeys[0]), AutoSealIntervalSecondsOpt(1))
	seed.GiveKey(t, minerNode, 0)
	mineraddr, minerOwnerAddr := seed.GiveMiner(t, minerNode, 0)
	_, err := storage.NewMiner(ctx, mineraddr, minerOwnerAddr, minerNode)
	assertions.NoError(err)
	clientNode := MakeNodeWithChainSeed(t, seed)
	nodes := []*Node{minerNode, clientNode}
	return mineraddr, nodes
}
