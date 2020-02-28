package node_test

import (
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
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
	_, nodes, fakeClock, blockTime := makeNodesBlockPropTests(t, numNodes)

	StartNodes(t, nodes)
	defer StopNodes(nodes)

	minerNode := nodes[0]

	connect(t, minerNode, nodes[1])
	connect(t, nodes[1], nodes[2])
	connect(t, nodes[2], nodes[3])

	// Advance node's time so that it is epoch 1
	fakeClock.Advance(blockTime)
	nextBlk, err := minerNode.BlockMining.BlockMiningAPI.MiningOnce(ctx)
	require.NoError(t, err)
	// Wait for network connection notifications to propagate
	time.Sleep(time.Millisecond * 100)

	equal := false
	for i := 0; i < 30; i++ {
		for j := 1; j < numNodes; j++ {
			otherHead := nodes[j].PorcelainAPI.ChainHeadKey()
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
	_, nodes, fakeClock, blockTime := makeNodesBlockPropTests(t, 2)

	StartNodes(t, nodes)
	defer StopNodes(nodes)

	connect(t, nodes[0], nodes[1])

	// Advance node's time so that it is epoch 1
	fakeClock.Advance(blockTime)
	_, err := nodes[0].BlockMining.BlockMiningAPI.MiningOnce(ctx)
	require.NoError(t, err)
	fakeClock.Advance(blockTime)
	_, err = nodes[0].BlockMining.BlockMiningAPI.MiningOnce(ctx)
	require.NoError(t, err)
	fakeClock.Advance(blockTime)
	thirdBlock, err := nodes[0].BlockMining.BlockMiningAPI.MiningOnce(ctx)
	require.NoError(t, err)

	equal := false
	for i := 0; i < 30; i++ {
		otherHead := nodes[1].PorcelainAPI.ChainHeadKey()
		assert.NotNil(t, otherHead)
		equal = otherHead.ToSlice()[0].Equals(thirdBlock.Cid())
		if equal {
			break
		}
		time.Sleep(time.Millisecond * 20)
	}

	assert.True(t, equal, "failed to sync chains")
}

// makeNodes makes at least two nodes, a miner and a client; numNodes is the total wanted
func makeNodesBlockPropTests(t *testing.T, numNodes int) (address.Address, []*Node, th.FakeClock, time.Duration) {
	seed := MakeChainSeed(t, MakeTestGenCfg(t, 3))
	ctx := context.Background()
	fc := th.NewFakeClock(time.Unix(1234567890, 0))
	blockTime := 100 * time.Millisecond
	c := clock.NewChainClockFromClock(1234567890, 100*time.Millisecond, fc)
	builder := test.NewNodeBuilder(t)
	builder.WithGenesisInit(seed.GenesisInitFunc)
	builder.WithBuilderOpt(ChainClockConfigOption(c))
	builder.WithBuilderOpt(TestProofOption())
	builder.WithInitOpt(PeerKeyOpt(PeerKeys[0]))
	minerNode := builder.Build(ctx)
	seed.GiveKey(t, minerNode, 0)
	mineraddr, _ := seed.GiveMiner(t, minerNode, 0)

	nodes := []*Node{minerNode}

	nodeLimit := 1
	if numNodes > 2 {
		nodeLimit = numNodes
	}
	builder2 := test.NewNodeBuilder(t)
	builder2.WithGenesisInit(seed.GenesisInitFunc)
	builder2.WithBuilderOpt(ChainClockConfigOption(c))
	builder2.WithBuilderOpt(TestProofOption())

	for i := 0; i < nodeLimit; i++ {
		nodes = append(nodes, builder2.Build(ctx))
	}
	return mineraddr, nodes, fc, blockTime
}
