package functional

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/build/project"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/constants"
	"github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/iptbtester"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestBootstrapMineOnce(t *testing.T) {
	tf.FunctionalTest(t)

	ctx := context.Background()
	root := project.Root()

	tns, err := iptbtester.NewTestNodes(t, 1, nil)
	require.NoError(t, err)

	node0 := tns[0]

	// Setup first node, note: Testbed.Name() is the directory
	genConfigPath := filepath.Join(root, "fixtures/setup.json")
	genesis := iptbtester.RequireGenesisFromSetup(t, node0.Testbed.Name(), genConfigPath)
	genesis.SectorsDir = filepath.Join(node0.Dir(), "sectors")
	genesis.PresealedSectorDir = filepath.Join(root, "./fixtures/genesis-sectors")

	node0.MustInitWithGenesis(ctx, genesis)
	node0.MustStart(ctx)
	defer node0.MustStop(ctx)

	var minerAddress address.Address
	node0.MustRunCmdJSON(ctx, &minerAddress, "go-filecoin", "config", "mining.minerAddress")

	// Check the miner's initial power corresponds to 2 2kb sectors
	var status porcelain.MinerStatus
	node0.MustRunCmdJSON(ctx, &status, "go-filecoin", "miner", "status", minerAddress.String())

	// expected miner power is 2 2kib sectors
	expectedMinerPower := constants.DevSectorSize * 2
	actualMinerPower := status.Power.Uint64()
	assert.Equal(t, uint64(expectedMinerPower), status.Power.Uint64(), "expected miner power: %d actual miner power: %d", expectedMinerPower, actualMinerPower)

	// Assert that the chain head is genesis block
	var blocks []block.Block
	node0.MustRunCmdJSON(ctx, &blocks, "go-filecoin", "chain", "ls")
	require.Equal(t, 1, len(blocks))
	assert.Equal(t, abi.ChainEpoch(0), blocks[0].Height)
	assert.Equal(t, big.Zero(), blocks[0].ParentWeight)
	assert.True(t, blocks[0].Parents.Equals(block.NewTipSetKey()))
	assert.Equal(t, builtin.SystemActorAddr, blocks[0].Miner)

	// Mine once
	node0.MustRunCmd(ctx, "go-filecoin", "mining", "once")

	// Assert that the chain head has now been mined by the miner
	node0.MustRunCmdJSON(ctx, &blocks, "go-filecoin", "chain", "ls")
	require.Equal(t, 1, len(blocks))
	assert.Equal(t, minerAddress, blocks[0].Miner)
}

func TestBootstrapWindowedPoSt(t *testing.T) {
	tf.FunctionalTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wd, _ := os.Getwd()
	genCfgPath := filepath.Join(wd, "..", "fixtures/setup.json")
	presealPath := filepath.Join(wd, "..", "fixtures/genesis-sectors")
	genTime := int64(1000000000)
	blockTime := 1 * time.Second
	fakeClock := clock.NewFake(time.Unix(genTime, 0))

	// Load genesis config fixture.
	genCfg := loadGenesisConfig(t, genCfgPath)
	// set proving period start to something soon
	start := abi.ChainEpoch(1)
	genCfg.Miners[0].ProvingPeriodStart = &start

	seed := node.MakeChainSeed(t, genCfg)
	chainClock := clock.NewChainClockFromClock(uint64(genTime), blockTime, fakeClock)

	miner := makeNode(ctx, t, seed, chainClock)
	_, _, err := initNodeGenesisMiner(t, miner, seed, genCfg.Miners[0].Owner, presealPath, genCfg.Miners[0].SectorSize)
	require.NoError(t, err)

	err = miner.Start(ctx)
	require.NoError(t, err)

	err = miner.StorageMining.Start(ctx)
	require.NoError(t, err)

	maddr, err := miner.BlockMining.BlockMiningAPI.MinerAddress()
	require.NoError(t, err)

	status, err := miner.PorcelainAPI.MinerGetStatus(ctx, maddr, requireChainHead(t, miner))
	require.NoError(t, err)

	require.Equal(t, abi.ChainEpoch(1), status.ProvingPeriodStart)

	// mine once to enter proving period
	fakeClock.Advance(blockTime)
	_, err = miner.BlockMining.BlockMiningAPI.MiningOnce(ctx)
	require.NoError(t, err)

	// Post should have been triggered, simulate mining while waiting for update to proving period start
	for i := 0; i < 25; i++ {
		fakeClock.Advance(blockTime)
		_, err := miner.BlockMining.BlockMiningAPI.MiningOnce(ctx)
		require.NoError(t, err)

		status, err := miner.PorcelainAPI.MinerGetStatus(ctx, maddr, requireChainHead(t, miner))
		require.NoError(t, err)

		if status.ProvingPeriodStart > 1 {
			return
		}

		time.Sleep(2 * time.Second)
	}
	t.Fatal("Timouut wating for windowed PoSt")
}
