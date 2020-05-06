package functional

import (
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/drand"
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestSingleMiner(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()
	genTime := int64(1000000000)
	blockTime := 30 * time.Second
	// The clock is intentionally set some way ahead of the genesis time so the miner can produce
	// catch-up blocks as quickly as possible.
	fakeClock := clock.NewFake(time.Unix(genTime, 0).Add(4 * time.Hour))

	// The fixture is needed in order to use the presealed genesis sectors fixture.
	// Future code could decouple the whole setup.json from the presealed information.
	genCfg := loadGenesisConfig(t, fixtureGenCfg())
	seed := node.MakeChainSeed(t, genCfg)
	chainClock := clock.NewChainClockFromClock(uint64(genTime), blockTime, fakeClock)

	drandImpl := &drand.Fake{
		GenesisTime:   time.Unix(genTime, 0).Add(-1 * blockTime),
		FirstFilecoin: 0,
	}

	nd := makeNode(ctx, t, seed, chainClock, drandImpl)
	minerAddr, _, err := initNodeGenesisMiner(ctx, t, nd, seed, genCfg.Miners[0].Owner, fixturePresealPath())
	require.NoError(t, err)

	err = nd.Start(ctx)
	require.NoError(t, err)
	defer nd.Stop(ctx)

	// Inspect genesis state.
	chainReader := nd.Chain().ChainReader
	head := block.NewTipSetKey(chainReader.GenesisCid())
	assert.Assert(t, chainReader.GetHead().Equals(head))

	// Mine a block.
	blk, err := nd.BlockMining.BlockMiningAPI.MiningOnce(ctx)
	require.NoError(t, err)
	assert.Equal(t, abi.ChainEpoch(1), blk.Height)
	assert.Assert(t, head.Equals(blk.Parents))
	assert.Equal(t, minerAddr, blk.Miner)
	assert.Assert(t, int64(blk.Timestamp) >= genTime+30)
	assert.Assert(t, int64(blk.Timestamp) < genTime+60)
	head = block.NewTipSetKey(blk.Cid())

	// Inspect chain state.
	assert.Assert(t, chainReader.GetHead().Equals(head))

	// Mine some more and expect a connected chain.
	for i := 2; i <= 5; i++ {
		blk, err = nd.BlockMining.BlockMiningAPI.MiningOnce(ctx)
		require.NoError(t, err)
		assert.Assert(t, head.Equals(blk.Parents))
		assert.Equal(t, abi.ChainEpoch(i), blk.Height)
		head = block.NewTipSetKey(blk.Cid())
	}
}

func TestSyncFromSingleMiner(t *testing.T) {
	tf.IntegrationTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	genTime := int64(1000000000)
	blockTime := 30 * time.Second
	fakeClock := clock.NewFake(time.Unix(genTime, 0))

	drandImpl := &drand.Fake{
		GenesisTime:   time.Unix(genTime, 0).Add(-1 * blockTime),
		FirstFilecoin: 0,
	}

	genCfg := loadGenesisConfig(t, fixtureGenCfg())
	seed := node.MakeChainSeed(t, genCfg)
	chainClock := clock.NewChainClockFromClock(uint64(genTime), blockTime, fakeClock)
	assert.Equal(t, fakeClock.Now(), chainClock.Now())

	ndMiner := makeNode(ctx, t, seed, chainClock, drandImpl)
	_, _, err := initNodeGenesisMiner(ctx, t, ndMiner, seed, genCfg.Miners[0].Owner, fixturePresealPath())
	require.NoError(t, err)

	ndValidator := makeNode(ctx, t, seed, chainClock, drandImpl)

	err = ndMiner.Start(ctx)
	require.NoError(t, err)
	err = ndValidator.Start(ctx)
	require.NoError(t, err)
	defer ndMiner.Stop(ctx)
	defer ndValidator.Stop(ctx)

	node.ConnectNodes(t, ndValidator, ndMiner)

	// Check the nodes are starting in the same place.
	head := ndMiner.Chain().ChainReader.GetHead()
	assert.Assert(t, ndValidator.Chain().ChainReader.GetHead().Equals(head))

	// Mine some blocks.
	for i := 1; i <= 3; i++ {
		fakeClock.Advance(blockTime)
		blk, err := ndMiner.BlockMining.BlockMiningAPI.MiningOnce(ctx)
		require.NoError(t, err)
		assert.Equal(t, abi.ChainEpoch(i), blk.Height)
		head = block.NewTipSetKey(blk.Cid())
	}

	// Inspect validator node chain state.
	require.NoError(t, th.WaitForIt(50, 20*time.Millisecond, func() (bool, error) {
		return ndValidator.Chain().ChainReader.GetHead().Equals(head), nil
	}), "validator failed to sync new head")
}

func TestBootstrapWindowedPoSt(t *testing.T) {
	// This test can require up to a whole proving period to elapse, which is slow even with fake proofs.
	tf.FunctionalTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	genTime := int64(1000000000)
	blockTime := 30 * time.Second
	fakeClock := clock.NewFake(time.Unix(genTime, 0))

	// Load genesis config fixture.
	genCfg := loadGenesisConfig(t, fixtureGenCfg())
	// set proving period start to something soon
	start := abi.ChainEpoch(0)
	genCfg.Miners[0].ProvingPeriodStart = &start

	seed := node.MakeChainSeed(t, genCfg)

	// fake proofs so we can run through a proving period quickly
	miner := test.NewNodeBuilder(t).
		WithBuilderOpt(node.ChainClockConfigOption(clock.NewChainClockFromClock(uint64(genTime), blockTime, fakeClock))).
		WithGenesisInit(seed.GenesisInitFunc).
		WithBuilderOpt(node.DrandConfigOption(&drand.Fake{
			GenesisTime:   time.Unix(genTime, 0).Add(-1 * blockTime),
			FirstFilecoin: 0,
		})).
		WithBuilderOpt(node.VerifierConfigOption(&proofs.FakeVerifier{})).
		WithBuilderOpt(node.PoStGeneratorOption(&consensus.TestElectionPoster{})).
		Build(ctx)

	_, _, err := initNodeGenesisMiner(ctx, t, miner, seed, genCfg.Miners[0].Owner, fixturePresealPath())
	require.NoError(t, err)

	err = miner.Start(ctx)
	require.NoError(t, err)

	err = miner.StorageMining.Start(ctx)
	require.NoError(t, err)

	// mine once to enter proving period
	go simulateBlockMining(ctx, t, fakeClock, blockTime, miner)

	minerAddr := miner.Repo.Config().Mining.MinerAddress

	// Post should have been triggered, simulate mining while waiting for update to proving period start
	for i := 0; i < 50; i++ {
		head := miner.Chain().ChainReader.GetHead()

		view, err := miner.Chain().State.StateView(head)
		require.NoError(t, err)

		poSts, err := view.MinerSuccessfulPoSts(ctx, minerAddr)
		require.NoError(t, err)

		if poSts > 0 {
			return
		}

		// We need to mine enough blocks to get to get to the deadline that contains our sectors. Add some friction here.
		time.Sleep(2 * time.Second)
	}
	t.Fatal("Timouut waiting for windowed PoSt")
}
