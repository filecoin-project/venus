package functional

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/build/project"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
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
	assert.Equal(t, address.Undef, blocks[0].Miner)

	// Mine once
	node0.MustRunCmd(ctx, "go-filecoin", "mining", "once")

	// Assert that the chain head has now been mined by the miner
	node0.MustRunCmdJSON(ctx, &blocks, "go-filecoin", "chain", "ls")
	require.Equal(t, 1, len(blocks))
	assert.Equal(t, minerAddress, blocks[0].Miner)
}

func TestBootstrapWindowedPoSt(t *testing.T) {
	//tf.FunctionalTest(t)

	ctx := context.Background()
	root := project.Root()

	tns, err := iptbtester.NewTestNodes(t, 1, nil)
	require.NoError(t, err)

	node0 := tns[0]

	// Setup first node, note: Testbed.Name() is the directory
	genConfigPath := filepath.Join(root, "fixtures/setup.json")
	genesisConfig := iptbtester.ReadSetup(t, genConfigPath)

	// set proving period start to something soon
	start := abi.ChainEpoch(97408490)
	genesisConfig.Miners[0].ProvingPeriodStart = &start

	genesis := iptbtester.RequireGenesis(t, node0.Testbed.Name(), genesisConfig)
	genesis.SectorsDir = filepath.Join(node0.Dir(), "sectors")
	genesis.PresealedSectorDir = filepath.Join(root, "./fixtures/genesis-sectors")

	node0.MustInitWithGenesis(ctx, genesis)
	node0.MustStart(ctx)
	defer node0.MustStop(ctx)

	var minerAddress address.Address
	node0.MustRunCmdJSON(ctx, &minerAddress, "go-filecoin", "config", "mining.minerAddress")

	// expect proving period start to be 1 and no failures
	var status porcelain.MinerStatus
	node0.MustRunCmdJSON(ctx, &status, "go-filecoin", "miner", "status", minerAddress.String())
	require.Equal(t, abi.ChainEpoch(97408490), status.ProvingPeriodStart)
	require.Equal(t, 0, status.PoStFailureCount)

	// start mining
	node0.MustRunCmd(ctx, "go-filecoin", "mining", "start")

	// Wait for either a changes the proving period start showing a successful PoSt or we see a failure on chain
	for i := 0; i < 30; i++ {
		node0.MustRunCmdJSON(ctx, &status, "go-filecoin", "miner", "status", minerAddress.String())
		require.Equal(t, 0, status.PoStFailureCount, "No PoSt received in proving period window")
		if status.ProvingPeriodStart > 97408490 {
			t.Log("Successfully advanced proving period start without incrementing failure count")
			return
		}
		time.Sleep(5 * time.Second)
	}
	t.Error("Time out waiting for PoSt to update")
}
