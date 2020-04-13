package functional

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	gengen "github.com/filecoin-project/go-filecoin/tools/gengen/util"
	"github.com/filecoin-project/sector-storage/ffiwrapper"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/stretchr/testify/require"
)

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
		WithBuilderOpt(node.VerifierConfigOption(ffiwrapper.ProofVerifier)).
		Build(ctx)
}

func initNodeGenesisMiner(t *testing.T, nd *node.Node, seed *node.ChainSeed, minerIdx int, presealPath string, sectorSize abi.SectorSize) (address.Address, address.Address, error) {
	seed.GiveKey(t, nd, minerIdx)
	miner, owner := seed.GiveMiner(t, nd, 0)

	err := node.ImportPresealedSectors(nd.Repo, presealPath, sectorSize, true)
	require.NoError(t, err)
	return miner, owner, err
}

func simulateBlockMining(ctx context.Context, t *testing.T, fakeClock clock.Fake, blockTime time.Duration, node *node.Node) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			fakeClock.Advance(blockTime)
			_, err := node.BlockMining.BlockMiningAPI.MiningOnce(ctx)
			require.NoError(t, err)
		}
	}
}

func requireChainHead(t *testing.T, node *node.Node) block.TipSetKey {
	tsk, err := node.PorcelainAPI.ChainHead()
	require.NoError(t, err)
	return tsk.Key()
}
