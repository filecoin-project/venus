package test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/build/project"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/constants"
	"github.com/filecoin-project/go-filecoin/internal/pkg/drand"
	gengen "github.com/filecoin-project/go-filecoin/tools/gengen/util"
)

const blockTime = builtin.EpochDurationSeconds * time.Second

// MustCreateNodesWithBootstrap creates an in-process test setup capable of testing communication between nodes.
// Every setup will have one bootstrap node (the first node that is called) that is setup to have power to mine.
// All of the proofs for the set-up are fake (but additional nodes will still need to create miners and add storage to
// gain power). All nodes will be started and connected to each other. The returned cancel function ensures all nodes
// are stopped when the test is over.
func MustCreateNodesWithBootstrap(ctx context.Context, t *testing.T, additionalNodes uint) ([]*node.Node, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)
	nodes := make([]*node.Node, 1+additionalNodes)

	// create bootstrap miner
	seed, genCfg, fakeClock, chainClock := CreateBootstrapSetup(t)
	nodes[0] = CreateBootstrapMiner(ctx, t, seed, chainClock, genCfg)

	// create additional nodes
	dr, _ := drand.RandomSchedule(time.Unix(0, 0))
	for i := uint(0); i < additionalNodes; i++ {
		node := NewNodeBuilder(t).
			WithGenesisInit(seed.GenesisInitFunc).
			WithConfig(node.DefaultAddressConfigOpt(seed.Addr(t, int(i+1)))).
			WithBuilderOpt(node.PoStGeneratorOption(&consensus.TestElectionPoster{})).
			WithBuilderOpt(node.FakeProofVerifierBuilderOpts()...).
			WithBuilderOpt(node.ChainClockConfigOption(chainClock)).
			WithBuilderOpt(node.DrandConfigOption(dr)).
			Build(ctx)
		addr := seed.GiveKey(t, node, int(i+1))
		err := node.PorcelainAPI.ConfigSet("wallet.defaultAddress", addr.String())
		require.NoError(t, err)
		err = node.Start(ctx)
		require.NoError(t, err)
		nodes[i+1] = node
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
				return
			default:
				RequireMineOnce(ctx, t, fakeClock, nodes[0])
			}
		}
	}()

	return nodes, cancel
}

func RequireMineOnce(ctx context.Context, t *testing.T, fakeClock clock.Fake, node *node.Node) *block.Block {
	fakeClock.Advance(blockTime)
	blk, err := node.BlockMining.BlockMiningAPI.MiningOnce(ctx)

	// fail only if ctx not done
	select {
	case <-ctx.Done():
		return nil
	default:
		require.NoError(t, err)
	}

	return blk
}

func CreateBootstrapSetup(t *testing.T) (*node.ChainSeed, *gengen.GenesisCfg, clock.Fake, clock.ChainEpochClock) {
	// set up paths and fake clock.
	genTime := int64(1000000000)
	fakeClock := clock.NewFake(time.Unix(genTime, 0))
	propDelay := 6 * time.Second

	// Load genesis config fixture.
	genCfgPath := project.Root("fixtures/setup.json")
	genCfg := loadGenesisConfig(t, genCfgPath)
	genCfg.Miners = append(genCfg.Miners, &gengen.CreateStorageMinerConfig{
		Owner:         5,
		SealProofType: constants.DevSealProofType,
	})
	seed := node.MakeChainSeed(t, genCfg)
	chainClock := clock.NewChainClockFromClock(uint64(genTime), blockTime, propDelay, fakeClock)

	return seed, genCfg, fakeClock, chainClock
}

func CreateBootstrapMiner(ctx context.Context, t *testing.T, seed *node.ChainSeed, chainClock clock.ChainEpochClock, genCfg *gengen.GenesisCfg) *node.Node {
	// set up paths and fake clock.
	presealPath := project.Root("fixtures/genesis-sectors")
	minerAddress, err := address.NewIDAddress(106)
	require.NoError(t, err)

	// create bootstrap miner
	dr, _ := drand.RandomSchedule(time.Unix(0, 0))
	bootstrapMiner := NewNodeBuilder(t).
		WithGenesisInit(seed.GenesisInitFunc).
		WithBuilderOpt(node.FakeProofVerifierBuilderOpts()...).
		WithBuilderOpt(node.PoStGeneratorOption(&consensus.TestElectionPoster{})).
		WithBuilderOpt(node.ChainClockConfigOption(chainClock)).
		WithBuilderOpt(node.DrandConfigOption(dr)).
		WithBuilderOpt(node.MonkeyPatchSetProofTypeOption(constants.DevRegisteredSealProof)).
		WithConfig(func(c *config.Config) {
			c.SectorBase.PreSealedSectorsDirPath = presealPath
			c.Mining.MinerAddress = minerAddress
		}).
		Build(ctx)

	addr := seed.GiveKey(t, bootstrapMiner, 0)
	err = bootstrapMiner.PorcelainAPI.ConfigSet("wallet.defaultAddress", addr.String())
	require.NoError(t, err)

	_, _, err = initNodeGenesisMiner(ctx, t, bootstrapMiner, seed, genCfg.Miners[0].Owner)
	require.NoError(t, err)
	err = bootstrapMiner.Start(ctx)
	require.NoError(t, err)

	return bootstrapMiner
}

func initNodeGenesisMiner(ctx context.Context, t *testing.T, nd *node.Node, seed *node.ChainSeed, minerIdx int) (address.Address, address.Address, error) {
	seed.GiveKey(t, nd, minerIdx)
	miner, owner := seed.GiveMiner(t, nd, 0)

	genesisBlock, err := nd.Chain().ChainReader.GetGenesisBlock(ctx)
	require.NoError(t, err)

	err = node.InitSectors(ctx, nd.Repo, genesisBlock)
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
