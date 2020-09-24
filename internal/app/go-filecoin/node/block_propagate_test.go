package node_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	specsbig "github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/constants"
	// "github.com/filecoin-project/go-filecoin/internal/pkg/drand"
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/version"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
	gengen "github.com/filecoin-project/go-filecoin/tools/gengen/util"
)

func TestBlockPropsManyNodes(t *testing.T) {
	tf.IntegrationTest(t)

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

func TestChainSyncA(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()
	_, nodes, fakeClock, blockTime := makeNodesBlockPropTests(t, 2)

	StartNodes(t, nodes)
	defer StopNodes(nodes)

	ConnectNodes(t, nodes[0], nodes[1])

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
		time.Sleep(time.Millisecond * 50)
	}

	assert.True(t, equal, "failed to sync chains")
}

func TestChainSyncWithMessages(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()

	/* setup */
	// genesis has two accounts
	genCfg := &gengen.GenesisCfg{}
	require.NoError(t, gengen.MinerConfigs(MakeTestGenCfg(t, 1).Miners)(genCfg))
	require.NoError(t, gengen.GenKeys(3, "1000000")(genCfg))
	require.NoError(t, gengen.NetworkName(version.TEST)(genCfg))
	cs := MakeChainSeed(t, genCfg)
	genUnixSeconds := int64(1234567890)
	genTime := time.Unix(genUnixSeconds, 0)
	fakeClock := clock.NewFake(genTime)
	blockTime := 30 * time.Second
	propDelay := 6 * time.Second
	c := clock.NewChainClockFromClock(uint64(genUnixSeconds), blockTime, propDelay, fakeClock)

	// first node is the message sender.
	builder1 := test.NewNodeBuilder(t).
		WithBuilderOpt(ChainClockConfigOption(c)).
		WithGenesisInit(cs.GenesisInitFunc).
		WithBuilderOpt(VerifierConfigOption(&proofs.FakeVerifier{})).
		WithBuilderOpt(MonkeyPatchSetProofTypeOption(constants.DevRegisteredSealProof))
		//WithBuilderOpt(DrandConfigOption(drand.NewFake(genTime)))
	nodeSend := builder1.Build(ctx)
	senderAddress := cs.GiveKey(t, nodeSend, 1)

	// second node is receiver
	builder2 := test.NewNodeBuilder(t).
		WithBuilderOpt(ChainClockConfigOption(c)).
		WithGenesisInit(cs.GenesisInitFunc).
		WithBuilderOpt(VerifierConfigOption(&proofs.FakeVerifier{})).
		WithBuilderOpt(MonkeyPatchSetProofTypeOption(constants.DevRegisteredSealProof))
		//WithBuilderOpt(DrandConfigOption(drand.NewFake(genTime)))
	nodeReceive := builder2.Build(ctx)
	receiverAddress := cs.GiveKey(t, nodeReceive, 2)

	// third node is miner
	builder3 := test.NewNodeBuilder(t).
		WithBuilderOpt(ChainClockConfigOption(c)).
		WithGenesisInit(cs.GenesisInitFunc).
		WithBuilderOpt(VerifierConfigOption(&proofs.FakeVerifier{})).
		WithBuilderOpt(MonkeyPatchSetProofTypeOption(constants.DevRegisteredSealProof)).
		WithBuilderOpt(PoStGeneratorOption(&consensus.TestElectionPoster{}))
		//WithBuilderOpt(DrandConfigOption(drand.NewFake(genTime)))
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
	gasCap := types.NewGasFeeCap(1)
	gasPremium := types.NewGasPremium(1)
	// ToDo
	// expGasCost := gas.NewGas(242).ToTokens(gasPrice) // DRAGONS -- this is brittle need a better way to predict this.
	expGasCost := gas.NewGas(242).AsBigInt()

	/* send message from SendNode */
	sendVal := specsbig.NewInt(100)
	_, _, err = nodeSend.PorcelainAPI.MessageSend(
		ctx,
		senderAddress,
		receiverAddress,
		sendVal,
		gasCap,
		gasPremium,
		gas.NewGas(1000),
		builtin.MethodSend,
		[]byte{},
	)
	require.NoError(t, err)
	smsgs, err := nodeMine.PorcelainAPI.MessagePoolWait(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 1, len(smsgs))
	uCid, err := smsgs[0].Message.Cid() // Message waiter needs unsigned cid for bls
	require.NoError(t, err)

	/* mine block with message */
	fakeClock.Advance(blockTime)
	fmt.Printf("about to mining once\n")
	_, err = nodeMine.BlockMining.BlockMiningAPI.MiningOnce(ctx)
	require.NoError(t, err)
	fmt.Printf("finished mining once\n")
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
func makeNodesBlockPropTests(t *testing.T, numNodes int) (address.Address, []*Node, clock.Fake, time.Duration) {
	seed := MakeChainSeed(t, MakeTestGenCfg(t, 3))
	ctx := context.Background()
	genUnixSeconds := int64(1234567890)
	genTime := time.Unix(genUnixSeconds, 0)
	fc := clock.NewFake(genTime)
	blockTime := 30 * time.Second
	propDelay := 6 * time.Second
	c := clock.NewChainClockFromClock(1234567890, blockTime, propDelay, fc)

	builder := test.NewNodeBuilder(t).
		WithGenesisInit(seed.GenesisInitFunc).
		WithBuilderOpt(ChainClockConfigOption(c)).
		WithBuilderOpt(VerifierConfigOption(&proofs.FakeVerifier{})).
		WithBuilderOpt(PoStGeneratorOption(&consensus.TestElectionPoster{})).
		WithBuilderOpt(MonkeyPatchSetProofTypeOption(constants.DevRegisteredSealProof)).
		//WithBuilderOpt(DrandConfigOption(drand.NewFake(genTime))).
		WithInitOpt(PeerKeyOpt(PeerKeys[0]))
	minerNode := builder.Build(ctx)
	seed.GiveKey(t, minerNode, 0)
	mineraddr, _ := seed.GiveMiner(t, minerNode, 0)

	nodes := []*Node{minerNode}

	nodeLimit := 1
	if numNodes > 2 {
		nodeLimit = numNodes
	}
	builder2 := test.NewNodeBuilder(t).
		WithGenesisInit(seed.GenesisInitFunc).
		WithBuilderOpt(ChainClockConfigOption(c)).
		WithBuilderOpt(VerifierConfigOption(&proofs.FakeVerifier{})).
		WithBuilderOpt(PoStGeneratorOption(&consensus.TestElectionPoster{})).
		WithBuilderOpt(MonkeyPatchSetProofTypeOption(constants.DevRegisteredSealProof))
		//WithBuilderOpt(DrandConfigOption(drand.NewFake(genTime)))

	for i := 0; i < nodeLimit; i++ {
		nodes = append(nodes, builder2.Build(ctx))
	}
	return mineraddr, nodes, fc, blockTime
}
