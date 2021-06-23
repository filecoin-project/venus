package test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/build/project"
	"github.com/filecoin-project/venus/pkg/clock"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/specactors/builtin"
	gengen "github.com/filecoin-project/venus/tools/gengen/util"
)

const blockTime = builtin.EpochDurationSeconds * time.Second

func CreateBootstrapSetup(t *testing.T) (*ChainSeed, *gengen.GenesisCfg, clock.ChainEpochClock) {
	// set up paths and fake clock.
	genTime := int64(1000000000)
	fakeClock := clock.NewFake(time.Unix(genTime, 0))

	// Load genesis config fixture.
	genCfgPath := project.Root("fixtures/setup.json")
	genCfg := loadGenesisConfig(t, genCfgPath)
	genCfg.Miners = append(genCfg.Miners, &gengen.CreateStorageMinerConfig{
		Owner:         5,
		SealProofType: constants.DevSealProofType,
	})
	seed := MakeChainSeed(t, genCfg)
	chainClock := clock.NewChainClockFromClock(uint64(genTime), blockTime, fakeClock)

	return seed, genCfg, chainClock
}

func CreateBootstrapMiner(ctx context.Context, t *testing.T, seed *ChainSeed, chainClock clock.ChainEpochClock, genCfg *gengen.GenesisCfg) *node.Node {
	// create bootstrap miner
	bootstrapMiner := NewNodeBuilder(t).
		WithGenesisInit(seed.GenesisInitFunc).
		WithBuilderOpt(FakeProofVerifierBuilderOpts()...).
		WithBuilderOpt(node.ChainClockConfigOption(chainClock)).
		WithBuilderOpt(node.MonkeyPatchSetProofTypeOption(constants.DevRegisteredSealProof)).
		Build(ctx)

	addr := seed.GiveKey(t, bootstrapMiner, 0)
	err := bootstrapMiner.ConfigModule().API().ConfigSet(ctx, "walletModule.defaultAddress", addr.String())
	require.NoError(t, err)

	_, _, err = initNodeGenesisMiner(ctx, t, bootstrapMiner, seed, genCfg.Miners[0].Owner)
	require.NoError(t, err)
	err = bootstrapMiner.Start(ctx)
	require.NoError(t, err)

	return bootstrapMiner
}

func initNodeGenesisMiner(ctx context.Context, t *testing.T, nd *node.Node, seed *ChainSeed, minerIdx int) (address.Address, address.Address, error) {
	seed.GiveKey(t, nd, minerIdx)
	miner, owner := seed.GiveMiner(t, nd, 0)

	return miner, owner, nil
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
