package node

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/protocol/storage"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func connect(t *testing.T, nd1, nd2 *Node) {
	t.Helper()
	pinfo := peer.AddrInfo{
		ID:    nd2.Host().ID(),
		Addrs: nd2.Host().Addrs(),
	}

	if err := nd1.Host().Connect(context.Background(), pinfo); err != nil {
		t.Fatal(err)
	}
}

func TestBlockPropsManyNodes(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	numNodes := 4
	minerAddr, nodes := makeNodes(t, numNodes)

	// Now add 10 null blocks and 1 tipset.
	signer, ki := types.NewMockSignersAndKeyInfo(1)
	mockSignerPubKey := ki[0].PublicKey()

	StartNodes(t, nodes)
	defer StopNodes(nodes)

	minerNode := nodes[0]

	connect(t, minerNode, nodes[1])
	connect(t, nodes[1], nodes[2])
	connect(t, nodes[2], nodes[3])

	head := minerNode.ChainReader.GetHead()
	headTipSet, err := minerNode.ChainReader.GetTipSet(head)
	require.NoError(t, err)
	baseTS := headTipSet
	require.NotNil(t, baseTS)
	proof := testhelpers.MakeRandomPoStProofForTest()

	ticket, err := signer.CreateTicket(proof, mockSignerPubKey)
	require.NoError(t, err)

	nextBlk := &types.Block{
		Miner:           minerAddr,
		Parents:         baseTS.Key(),
		Height:          types.Uint64(1),
		ParentWeight:    types.Uint64(10000),
		StateRoot:       baseTS.ToSlice()[0].StateRoot,
		Proof:           proof,
		Ticket:          ticket,
		Messages:        types.EmptyMessagesCID,
		MessageReceipts: types.EmptyReceiptsCID,
	}

	// Wait for network connection notifications to propagate
	time.Sleep(time.Millisecond * 300)

	assert.NoError(t, minerNode.AddNewBlock(ctx, nextBlk))

	equal := false
	for i := 0; i < 30; i++ {
		for j := 1; j < numNodes; j++ {
			otherHead := nodes[j].ChainReader.GetHead()
			assert.NotNil(t, otherHead)
			equal = otherHead.ToSlice()[0].Equals(nextBlk.Cid())
			if equal {
				break
			}
			time.Sleep(time.Millisecond * 20)
		}
	}

	assert.True(t, equal, "failed to sync chains")
}

func TestChainSync(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	minerAddr, nodes := makeNodes(t, 2)
	StartNodes(t, nodes)
	defer StopNodes(nodes)

	head := nodes[0].ChainReader.GetHead()
	headTipSet, err := nodes[0].ChainReader.GetTipSet(head)
	require.NoError(t, err)
	baseTS := headTipSet

	signer, ki := types.NewMockSignersAndKeyInfo(1)
	minerWorker, err := ki[0].Address()
	require.NoError(t, err)
	stateRoot := baseTS.ToSlice()[0].StateRoot
	msgsCid := types.EmptyMessagesCID
	rcptsCid := types.EmptyReceiptsCID

	nextBlk1 := testhelpers.NewValidTestBlockFromTipSet(baseTS, stateRoot, 1, minerAddr, minerWorker, signer)
	nextBlk1.Messages = msgsCid
	nextBlk1.MessageReceipts = rcptsCid
	nextBlk2 := testhelpers.NewValidTestBlockFromTipSet(baseTS, stateRoot, 2, minerAddr, minerWorker, signer)
	nextBlk2.Messages = msgsCid
	nextBlk2.MessageReceipts = rcptsCid
	nextBlk3 := testhelpers.NewValidTestBlockFromTipSet(baseTS, stateRoot, 3, minerAddr, minerWorker, signer)
	nextBlk3.Messages = msgsCid
	nextBlk3.MessageReceipts = rcptsCid

	assert.NoError(t, nodes[0].AddNewBlock(ctx, nextBlk1))
	assert.NoError(t, nodes[0].AddNewBlock(ctx, nextBlk2))
	assert.NoError(t, nodes[0].AddNewBlock(ctx, nextBlk3))

	connect(t, nodes[0], nodes[1])
	equal := false
	for i := 0; i < 30; i++ {
		otherHead := nodes[1].ChainReader.GetHead()
		assert.NotNil(t, otherHead)
		equal = otherHead.ToSlice()[0].Equals(nextBlk3.Cid())
		if equal {
			break
		}
		time.Sleep(time.Millisecond * 20)
	}

	assert.True(t, equal, "failed to sync chains")
}

type ZeroRewarder struct{}

func (r *ZeroRewarder) BlockReward(ctx context.Context, st state.Tree, minerAddr address.Address) error {
	return nil
}

func (r *ZeroRewarder) GasReward(ctx context.Context, st state.Tree, minerAddr address.Address, msg *types.SignedMessage, cost types.AttoFIL) error {
	return nil
}

// makeNodes makes at least two nodes, a miner and a client; numNodes is the total wanted
func makeNodes(t *testing.T, numNodes int) (address.Address, []*Node) {
	seed := MakeChainSeed(t, TestGenCfg)
	configOpts := []ConfigOpt{RewarderConfigOption(&ZeroRewarder{})}
	minerNode := MakeNodeWithChainSeed(t, seed, configOpts,
		PeerKeyOpt(PeerKeys[0]),
		AutoSealIntervalSecondsOpt(1),
	)
	seed.GiveKey(t, minerNode, 0)
	mineraddr, ownerAddr := seed.GiveMiner(t, minerNode, 0)
	_, err := storage.NewMiner(mineraddr, ownerAddr, ownerAddr, &storage.FakeProver{}, types.OneKiBSectorSize, minerNode, minerNode.Repo.DealsDatastore(), minerNode.PorcelainAPI)
	assert.NoError(t, err)

	nodes := []*Node{minerNode}

	nodeLimit := 1
	if numNodes > 2 {
		nodeLimit = numNodes
	}
	for i := 0; i < nodeLimit; i++ {
		nodes = append(nodes, MakeNodeWithChainSeed(t, seed, configOpts))
	}
	return mineraddr, nodes
}
