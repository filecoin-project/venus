package node_test

import (
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	specsbig "github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/version"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
	gengen "github.com/filecoin-project/go-filecoin/tools/gengen/util"
)

func TestBlockPropsManyNodes(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	numNodes := 4
	_, nodes, fakeClock, blockTime := makeNodesBlockPropTests(t, numNodes)

	StartNodes(t, nodes)
	defer StopNodes(nodes)

	minerNode := nodes[0]

	ConnectNodes(t, minerNode, nodes[1])
	ConnectNodes(t, nodes[1], nodes[2])
	ConnectNodes(t, nodes[2], nodes[3])

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

	ConnectNodes(t, nodes[0], nodes[1])

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

func TestChainSyncWithMessages(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()

	/* setup */
	// genesis has two accounts
	genCfg := &gengen.GenesisCfg{}
	require.NoError(t, gengen.MinerConfigs(MakeTestGenCfg(t, 1).Miners)(genCfg))
	require.NoError(t, gengen.GenKeys(3)(genCfg))
	require.NoError(t, gengen.GenKeyPrealloc(1, "100000")(genCfg))
	require.NoError(t, gengen.GenKeyPrealloc(2, "100")(genCfg))
	require.NoError(t, gengen.NetworkName(version.TEST)(genCfg))
	cs := MakeChainSeed(t, genCfg)
	fakeClock := th.NewFakeClock(time.Unix(1234567890, 0))
	blockTime := 30 * time.Second
	c := clock.NewChainClockFromClock(1234567890, blockTime, fakeClock)

	// first node is the message sender.
	builder1 := test.NewNodeBuilder(t).
		WithBuilderOpt(ChainClockConfigOption(c)).
		WithGenesisInit(cs.GenesisInitFunc).
		WithBuilderOpt(VerifierConfigOption(&proofs.FakeVerifier{}))
	nodeSend := builder1.Build(ctx)
	senderAddress := cs.GiveKey(t, nodeSend, 1)

	// second node is receiver
	builder2 := test.NewNodeBuilder(t).
		WithBuilderOpt(ChainClockConfigOption(c)).
		WithGenesisInit(cs.GenesisInitFunc).
		WithBuilderOpt(VerifierConfigOption(&proofs.FakeVerifier{}))
	nodeReceive := builder2.Build(ctx)
	receiverAddress := cs.GiveKey(t, nodeReceive, 2)

	// third node is miner
	builder3 := test.NewNodeBuilder(t).
		WithBuilderOpt(ChainClockConfigOption(c)).
		WithGenesisInit(cs.GenesisInitFunc).
		WithBuilderOpt(VerifierConfigOption(&proofs.FakeVerifier{})).
		WithBuilderOpt(PoStGeneratorOption(&consensus.TestElectionPoster{}))
	nodeMine := builder3.Build(ctx)
	cs.GiveKey(t, nodeMine, 0)
	cs.GiveMiner(t, nodeMine, 0)

	StartNodes(t, []*Node{nodeSend, nodeReceive, nodeMine})
	ConnectNodes(t, nodeSend, nodeMine)
	ConnectNodes(t, nodeMine, nodeSend)
	ConnectNodes(t, nodeMine, nodeReceive)
	ConnectNodes(t, nodeReceive, nodeMine)

	/* collect initial balance values */
	senderStart, err := nodeSend.PorcelainAPI.WalletBalance(ctx, senderAddress)
	require.NoError(t, err)
	receiverStart, err := nodeReceive.PorcelainAPI.WalletBalance(ctx, receiverAddress)
	require.NoError(t, err)
	gasPrice := types.NewGasPrice(1)
	expGasCost := gas.NewGas(242).ToTokens(gasPrice) // DRAGONS -- this is brittle need a better way to predict this.

	/* send message from SendNode */
	sendVal := specsbig.NewInt(100)
	_, _, err = nodeSend.PorcelainAPI.MessageSend(
		ctx,
		senderAddress,
		receiverAddress,
		sendVal,
		gasPrice,
		types.GasUnits(1000),
		builtin.MethodSend,
		&adt.EmptyValue{},
	)
	require.NoError(t, err)
	smsgs, err := nodeMine.PorcelainAPI.MessagePoolWait(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 1, len(smsgs))
	uCid, err := smsgs[0].Message.Cid() // Message waiter needs unsigned cid for bls
	require.NoError(t, err)

	/* mine block with message */
	fakeClock.Advance(blockTime)
	_, err = nodeMine.BlockMining.BlockMiningAPI.MiningOnce(ctx)
	require.NoError(t, err)

	/* verify new state */
	_, err = nodeReceive.PorcelainAPI.MessageWaitDone(ctx, uCid)
	require.NoError(t, err)
	_, err = nodeSend.PorcelainAPI.MessageWaitDone(ctx, uCid)
	require.NoError(t, err)

	senderEnd, err := nodeSend.PorcelainAPI.WalletBalance(ctx, senderAddress)
	require.NoError(t, err)
	receiverEnd, err := nodeReceive.PorcelainAPI.WalletBalance(ctx, receiverAddress)
	require.NoError(t, err)

	assert.Equal(t, senderStart, specsbig.Add(specsbig.Add(senderEnd, sendVal), expGasCost))
	assert.Equal(t, receiverEnd, specsbig.Add(receiverStart, sendVal))
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
	builder.WithBuilderOpt(VerifierConfigOption(&proofs.FakeVerifier{}))
	builder.WithBuilderOpt(PoStGeneratorOption(&consensus.TestElectionPoster{}))
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
	builder2.WithBuilderOpt(VerifierConfigOption(&proofs.FakeVerifier{}))
	builder2.WithBuilderOpt(PoStGeneratorOption(&consensus.TestElectionPoster{}))

	for i := 0; i < nodeLimit; i++ {
		nodes = append(nodes, builder2.Build(ctx))
	}
	return mineraddr, nodes, fc, blockTime
}
