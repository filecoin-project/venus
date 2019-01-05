package node

import (
	"context"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"testing"
	"time"

	"gx/ipfs/QmPiemjiKBC9VA7vZF82m4x1oygtg2c2YVqag8PX7dN1BD/go-libp2p-peerstore"

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
	proof := testhelpers.MakeRandomPoSTProofForTest()

	nextBlk := &types.Block{
		Miner:        minerAddr,
		Parents:      baseTS.ToSortedCidSet(),
		Height:       types.Uint64(1),
		ParentWeight: types.Uint64(10000),
		StateRoot:    baseTS.ToSlice()[0].StateRoot,
		Proof:        proof,
		Ticket:       consensus.CreateTicket(proof, minerAddr),
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
	//t.Parallel()
	ctx := context.Background()
	assert := assert.New(t)

	minerAddr, nodes := makeNodes(ctx, t, assert)
	startNodes(t, nodes)
	defer stopNodes(nodes)

	baseTS := nodes[0].ChainReader.Head()
	nextBlk1 := testhelpers.NewValidTestBlockFromTipSet(baseTS, 1, minerAddr)
	nextBlk2 := testhelpers.NewValidTestBlockFromTipSet(baseTS, 2, minerAddr)
	nextBlk3 := testhelpers.NewValidTestBlockFromTipSet(baseTS, 3, minerAddr)

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

type zeroRewarder struct{}

func (r *zeroRewarder) BlockReward(ctx context.Context, st state.Tree, minerAddr address.Address) error {
	return nil
}

func makeNodes(ctx context.Context, t *testing.T, assertions *assert.Assertions) (address.Address, []*Node) {
	seed := MakeChainSeed(t, TestGenCfg)
	configOpts := []ConfigOpt{RewarderConfigOption(&zeroRewarder{})}
	minerNode := MakeNodeWithChainSeed(t, seed, configOpts,
		PeerKeyOpt(PeerKeys[0]),
		AutoSealIntervalSecondsOpt(1),
	)
	seed.GiveKey(t, minerNode, 0)
	mineraddr, minerOwnerAddr := seed.GiveMiner(t, minerNode, 0)
	_, err := storage.NewMiner(ctx, mineraddr, minerOwnerAddr, minerNode, minerNode.PlumbingAPI)
	assertions.NoError(err)
	clientNode := MakeNodeWithChainSeed(t, seed, configOpts)
	nodes := []*Node{minerNode, clientNode}
	return mineraddr, nodes
}
