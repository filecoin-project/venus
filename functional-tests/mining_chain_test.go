package functional

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-sectorbuilder"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	gengen "github.com/filecoin-project/go-filecoin/tools/gengen/util"
)

func TestSingleMiner(t *testing.T) {
	tf.FunctionalTest(t)
	ctx := context.Background()
	wd, _ := os.Getwd()
	genCfgPath := filepath.Join(wd, "..", "fixtures/setup.json")
	presealPath := filepath.Join(wd, "..", "fixtures/genesis-sectors")
	genTime := int64(1000000000)
	blockTime := 30 * time.Second
	fakeClock := th.NewFakeClock(time.Unix(genTime, 0))

	// Load genesis config fixture.
	// The fixture is needed in order to use the presealed genesis sectors fixture.
	// Future code could decouple the whole setup.json from the presealed information.
	genCfg := loadGenesisConfig(t, genCfgPath)
	seed := node.MakeChainSeed(t, genCfg)
	chainClock := clock.NewChainClockFromClock(uint64(genTime), blockTime, fakeClock)

	nd := makeNode(ctx, t, seed, chainClock)
	minerAddr, _, err := initNodeGenesisMiner(t, nd, seed, genCfg.Miners[0].Owner, presealPath)
	require.NoError(t, err)

	err = nd.Start(ctx)
	require.NoError(t, err)

	// Inspect genesis state.
	chainReader := nd.Chain().ChainReader
	head := block.NewTipSetKey(chainReader.GenesisCid())
	assert.Assert(t, chainReader.GetHead().Equals(head))

	// Mine a block.
	fakeClock.Advance(blockTime)
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
		fakeClock.Advance(blockTime)
		blk, err = nd.BlockMining.BlockMiningAPI.MiningOnce(ctx)
		require.NoError(t, err)
		assert.Assert(t, head.Equals(blk.Parents))
		assert.Equal(t, abi.ChainEpoch(i), blk.Height)
		head = block.NewTipSetKey(blk.Cid())
	}

	nd.Stop(ctx)
}

func TestSyncFromSingleMiner(t *testing.T) {
	tf.FunctionalTest(t)
	ctx := context.Background()
	wd, _ := os.Getwd()
	genCfgPath := filepath.Join(wd, "..", "fixtures/setup.json")
	presealPath := filepath.Join(wd, "..", "fixtures/genesis-sectors")
	genTime := int64(1000000000)
	blockTime := 30 * time.Second
	fakeClock := th.NewFakeClock(time.Unix(genTime, 0))

	// Load genesis config fixture.
	// The fixture is needed in order to use the presealed genesis sectors fixture.
	// Future code could decouple the whole setup.json from the presealed information.
	genCfg := loadGenesisConfig(t, genCfgPath)
	seed := node.MakeChainSeed(t, genCfg)
	chainClock := clock.NewChainClockFromClock(uint64(genTime), blockTime, fakeClock)

	ndMiner := makeNode(ctx, t, seed, chainClock)
	_, _, err := initNodeGenesisMiner(t, ndMiner, seed, genCfg.Miners[0].Owner, presealPath)
	require.NoError(t, err)

	ndValidator := makeNode(ctx, t, seed, chainClock)

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

	// Inspect chain state.
	require.NoError(t, th.WaitForIt(50, 20*time.Millisecond, func() (bool, error) {
		return ndValidator.Chain().ChainReader.GetHead().Equals(head), nil
	}), "validator failed to sync new head")
}

func loadGenesisConfig(t *testing.T, path string) *gengen.GenesisCfg {
	configFile, err := os.Open(path)
	if err != nil {
		t.Errorf("failed to open config file %s: %s", path, err)
	}
	defer func() { _ = configFile.Close() }()

	var cfg gengen.GenesisCfg
	if err := json.NewDecoder(configFile).Decode(&cfg); err != nil {
		t.Errorf("failed to parse config: %s", err)
	}
	return &cfg
}

func makeNode(ctx context.Context, t *testing.T, seed *node.ChainSeed, chainClock clock.ChainEpochClock) *node.Node {
	return test.NewNodeBuilder(t).
		WithBuilderOpt(node.ChainClockConfigOption(chainClock)).
		WithGenesisInit(seed.GenesisInitFunc).
		WithBuilderOpt(node.VerifierConfigOption(sectorbuilder.ProofVerifier)).
		Build(ctx)
}

func initNodeGenesisMiner(t *testing.T, nd *node.Node, seed *node.ChainSeed, minerIdx int, presealPath string) (address.Address, address.Address, error) {
	seed.GiveKey(t, nd, minerIdx)
	miner, owner := seed.GiveMiner(t, nd, 0)

	err := node.ImportPresealedSectors(nd.Repo, presealPath, true)
	require.NoError(t, err)
	return miner, owner, err
}
