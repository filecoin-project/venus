package node

import (
	"context"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	peerstore "gx/ipfs/QmPiemjiKBC9VA7vZF82m4x1oygtg2c2YVqag8PX7dN1BD/go-libp2p-peerstore"

	"github.com/filecoin-project/go-filecoin/types"

	"github.com/stretchr/testify/assert"
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

	nodes := MakeNodesUnstarted(t, 2, false, true, nil)
	startNodes(t, nodes)
	defer stopNodes(nodes)
	connect(t, nodes[0], nodes[1])

	baseTS := nodes[0].ChainReader.Head()
	require.NotNil(t, baseTS)
	nextBlk := &types.Block{
		Parents:           baseTS.ToSortedCidSet(),
		Height:            types.Uint64(1),
		ParentWeightNum:   types.Uint64(10),
		ParentWeightDenom: types.Uint64(1),
		ParentWeight: types.Uint64(10000),
		StateRoot:         baseTS.ToSlice()[0].StateRoot,
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

	nodes := MakeNodesUnstarted(t, 2, false, true, nil)
	startNodes(t, nodes)
	defer stopNodes(nodes)

	baseTS := nodes[0].ChainReader.Head()
	stateRoot := baseTS.ToSlice()[0].StateRoot
	nextBlk1 := &types.Block{
		Parents:      baseTS.ToSortedCidSet(),
		Height:       types.Uint64(1),
		ParentWeight: types.Uint64(10000),
		StateRoot:    stateRoot,
	}
	nextBlk2 := &types.Block{
		Parents:      types.NewSortedCidSet(nextBlk1.Cid()),
		Height:       types.Uint64(2),
		ParentWeight: types.Uint64(20000),
		StateRoot:    stateRoot,
	}
	nextBlk3 := &types.Block{
		Parents:      types.NewSortedCidSet(nextBlk2.Cid()),
		Height:       types.Uint64(3),
		ParentWeight: types.Uint64(30000),
		StateRoot:    stateRoot,
	}
	nextBlk1 := consensus.NewValidTestBlockFromTipSet(baseTS, 1)
	nextBlk2 := consensus.NewValidTestBlockFromTipSet(baseTS, 2)
	nextBlk3 := consensus.NewValidTestBlockFromTipSet(baseTS, 3)

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
