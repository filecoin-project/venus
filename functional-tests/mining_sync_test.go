package functional

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/build/project"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
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

	kib := uint64(1024)
	// expected miner power is 2 2kib sectors
	expectedMinerPower := uint64((2 * kib) * 2)
	actualMinerPower := status.Power.Uint64()
	assert.Equal(t, expectedMinerPower, status.Power.Uint64(), "expected miner power: %d actual miner power: %d", expectedMinerPower, actualMinerPower)

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
