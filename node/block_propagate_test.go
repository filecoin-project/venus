package node

import (
	"context"
	"testing"
	"time"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
	"gx/ipfs/QmRhFARzTHcFh8wUxwN5KvyTGq73FLC65EfFAhz8Ng7aGb/go-libp2p-peerstore"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/protocol/storage"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
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

func TestBlockPropsManyNodes(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	assert := assert.New(t)

	numNodes := 4
	minerAddr, nodes := makeNodes(t, assert, numNodes)

	// Now add 10 null blocks and 1 tipset.
	signer, ki := types.NewMockSignersAndKeyInfo(1)
	mockSignerPubKey := ki[0].PublicKey()

	StartNodes(t, nodes)
	defer StopNodes(nodes)

	minerNode := nodes[0]

	connect(t, minerNode, nodes[1])
	connect(t, nodes[1], nodes[2])
	connect(t, nodes[2], nodes[3])

	baseTS := minerNode.ChainReader.Head()
	require.NotNil(t, baseTS)
	proof := testhelpers.MakeRandomPoSTProofForTest()

	ticket, err := signer.CreateTicket(proof, mockSignerPubKey)
	require.NoError(err)

	nextBlk := &types.Block{
		Miner:        minerAddr,
		Parents:      baseTS.ToSortedCidSet(),
		Height:       types.Uint64(1),
		ParentWeight: types.Uint64(10000),
		StateRoot:    baseTS.ToSlice()[0].StateRoot,
		Proof:        proof,
		Ticket:       ticket,
	}

	// Wait for network connection notifications to propagate
	time.Sleep(time.Millisecond * 300)

	assert.NoError(minerNode.AddNewBlock(ctx, nextBlk))

	equal := false
	for i := 0; i < 30; i++ {
		for j := 1; j < numNodes; j++ {
			otherHead := nodes[j].ChainReader.Head()
			assert.NotNil(t, otherHead)
			equal = otherHead.ToSlice()[0].Cid().Equals(nextBlk.Cid())
			if equal {
				break
			}
			time.Sleep(time.Millisecond * 20)
		}
	}

	assert.True(equal, "failed to sync chains")

}

func TestChainSync(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)

	minerAddr, nodes := makeNodes(t, assert, 2)
	StartNodes(t, nodes)
	defer StopNodes(nodes)

	baseTS := nodes[0].ChainReader.Head()

	signer, ki := types.NewMockSignersAndKeyInfo(1)
	mockSignerPubKey := ki[0].PublicKey()
	stateRoot := baseTS.ToSlice()[0].StateRoot

	nextBlk1 := testhelpers.NewValidTestBlockFromTipSet(baseTS, stateRoot, 1, minerAddr, mockSignerPubKey, signer)
	nextBlk2 := testhelpers.NewValidTestBlockFromTipSet(baseTS, stateRoot, 2, minerAddr, mockSignerPubKey, signer)
	nextBlk3 := testhelpers.NewValidTestBlockFromTipSet(baseTS, stateRoot, 3, minerAddr, mockSignerPubKey, signer)

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

type ZeroRewarder struct{}

func (r *ZeroRewarder) BlockReward(ctx context.Context, st state.Tree, minerAddr address.Address) error {
	return nil
}

func (r *ZeroRewarder) GasReward(ctx context.Context, st state.Tree, minerAddr address.Address, msg *types.SignedMessage, cost *types.AttoFIL) error {
	return nil
}

// makeNodes makes at least two nodes, a miner and a client; numNodes is the total wanted
func makeNodes(t *testing.T, assertions *assert.Assertions, numNodes int) (address.Address, []*Node) {
	seed := MakeChainSeed(t, TestGenCfg)
	configOpts := []ConfigOpt{RewarderConfigOption(&ZeroRewarder{})}
	minerNode := MakeNodeWithChainSeed(t, seed, configOpts,
		PeerKeyOpt(PeerKeys[0]),
		AutoSealIntervalSecondsOpt(1),
	)
	seed.GiveKey(t, minerNode, 0)
	mineraddr, minerOwnerAddr := seed.GiveMiner(t, minerNode, 0)
	_, err := storage.NewMiner(mineraddr, minerOwnerAddr, minerNode, minerNode.Repo.DealsDatastore(), minerNode.PorcelainAPI)
	assertions.NoError(err)

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
