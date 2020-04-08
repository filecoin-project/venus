package test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/build/project"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	gengen "github.com/filecoin-project/go-filecoin/tools/gengen/util"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/stretchr/testify/require"
)

func MustCreateNodesWithBootstrap(ctx context.Context, t *testing.T, additionalNodes uint) ([]*node.Node, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)
	nodes := make([]*node.Node, 1+additionalNodes)

	// set up paths and fake clock.
	presealPath := project.Root("fixtures/genesis-sectors")
	genCfgPath := project.Root("fixtures/setup.json")
	genTime := int64(1000000000)
	blockTime := 1 * time.Second
	fakeClock := clock.NewFake(time.Unix(genTime, 0))

	// Load genesis config fixture.
	genCfg := loadGenesisConfig(t, genCfgPath)
	genCfg.Miners = append(genCfg.Miners, &gengen.CreateStorageMinerConfig{
		Owner:      1,
		SectorSize: 2048,
	})
	seed := node.MakeChainSeed(t, genCfg)
	chainClock := clock.NewChainClockFromClock(uint64(genTime), blockTime, fakeClock)

	// create bootstrap miner
	bootstrapMiner := NewNodeBuilder(t).
		WithGenesisInit(seed.GenesisInitFunc).
		WithBuilderOpt(node.FakeProofVerifierBuilderOpts()...).
		WithBuilderOpt(node.ChainClockConfigOption(chainClock)).
		Build(ctx)

	_, _, err := initNodeGenesisMiner(t, bootstrapMiner, seed, genCfg.Miners[0].Owner, presealPath, genCfg.Miners[0].SectorSize)
	require.NoError(t, err)
	bootstrapMiner.Start(ctx)

	nodes[0] = bootstrapMiner

	// create addtiional nodes
	for i := uint(0); i < additionalNodes; i++ {
		nodes[i+1] = NewNodeBuilder(t).
			WithGenesisInit(seed.GenesisInitFunc).
			WithBuilderOpt(node.FakeProofVerifierBuilderOpts()...).
			WithBuilderOpt(node.ChainClockConfigOption(chainClock)).
			Build(ctx)
		addr := seed.GiveKey(t, nodes[i+1], int(i+1))
		nodes[i+1].PorcelainAPI.ConfigSet("wallet.defaultAddress", addr.String())
		nodes[i+1].Start(ctx)
	}

	// connect all nodes
	for i := 0; i < len(nodes); i++ {
		for j := 0; j < i; j++ {
			node.ConnectNodes(t, nodes[i], nodes[j])
		}
	}

	// start simulated mining and wait for shutdown
	go func() {
		for {
			select {
			case <-ctx.Done():
				for _, nd := range nodes {
					nd.Stop(ctx)
				}
				return
			default:
				fakeClock.Advance(blockTime)
				_, err := bootstrapMiner.BlockMining.BlockMiningAPI.MiningOnce(ctx)
				require.NoError(t, err)
			}
		}
	}()

	return nodes, cancel
}

func initNodeGenesisMiner(t *testing.T, nd *node.Node, seed *node.ChainSeed, minerIdx int, presealPath string, sectorSize abi.SectorSize) (address.Address, address.Address, error) {
	seed.GiveKey(t, nd, minerIdx)
	miner, owner := seed.GiveMiner(t, nd, 0)

	err := node.ImportPresealedSectors(nd.Repo, presealPath, sectorSize, true)
	require.NoError(t, err)
	return miner, owner, err
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
