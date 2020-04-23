package functional

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/pkg/drand"

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
	//tf.FunctionalTest(t)
	ctx := context.Background()
	wd, _ := os.Getwd()
	genCfgPath := filepath.Join(wd, "..", "fixtures/setup.json")
	presealPath := filepath.Join(wd, "..", "fixtures/genesis-sectors")
	genTime := int64(1000000000)
	blockTime := 30 * time.Second
	// The clock is intentionally set some way ahead of the genesis time so the miner can produce
	// catch-up blocks as quickly as possible.
	fakeClock := clock.NewFake(time.Unix(genTime, 0).Add(4 * time.Hour))

	// Load genesis config fixture.
	// The fixture is needed in order to use the presealed genesis sectors fixture.
	// Future code could decouple the whole setup.json from the presealed information.
	genCfg := loadGenesisConfig(t, genCfgPath)
	seed := node.MakeChainSeed(t, genCfg)
	chainClock := clock.NewChainClockFromClock(uint64(genTime), blockTime, fakeClock)

	drandImpl := &drand.Fake{
		GenesisTime:   time.Unix(int64(genTime), 0).Add(-1 * blockTime),
		FirstFilecoin: 0,
	}

	nd := makeNode(ctx, t, seed, chainClock, drandImpl)
	minerAddr, _, err := initNodeGenesisMiner(ctx, t, nd, seed, genCfg.Miners[0].Owner, presealPath)
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
	t.Skip("Unskip when we have implemented production drand component and local drand network for functional tests")

	tf.FunctionalTest(t)
	ctx := context.Background()
	wd, _ := os.Getwd()
	genCfgPath := filepath.Join(wd, "..", "fixtures/setup.json")
	presealPath := filepath.Join(wd, "..", "fixtures/genesis-sectors")
	genTime := int64(1000000000)
	blockTime := 30 * time.Second
	fakeClock := clock.NewFake(time.Unix(genTime, 0))

	// Load genesis config fixture.
	genCfg := loadGenesisConfig(t, genCfgPath)
	seed := node.MakeChainSeed(t, genCfg)
	chainClock := clock.NewChainClockFromClock(uint64(genTime), blockTime, fakeClock)
	assert.Equal(t, fakeClock.Now(), chainClock.Now())

	ndMiner := makeNode(ctx, t, seed, chainClock, nil)
	_, _, err := initNodeGenesisMiner(ctx, t, ndMiner, seed, genCfg.Miners[0].Owner, presealPath)
	require.NoError(t, err)

	ndValidator := makeNode(ctx, t, seed, chainClock, nil)

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
