package functional

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/internal/pkg/beacon"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/internal/app/go-filecoin/node"
	"github.com/filecoin-project/venus/internal/app/go-filecoin/node/test"
	"github.com/filecoin-project/venus/internal/pkg/clock"
	"github.com/filecoin-project/venus/internal/pkg/constants"
	gengen "github.com/filecoin-project/venus/tools/gengen/util"
)

// setup presealed sectors and use these paths to run test against sectors with larger sector size
//genCfgPath := filepath.Join("./512", "setup.json")
//presealPath := "./512"
func fixtureGenCfg() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "..", "fixtures/setup.json")
}

func fixturePresealPath() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "..", "fixtures/genesis-sectors")
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

func makeNode(ctx context.Context, t *testing.T, seed *node.ChainSeed, chainClock clock.ChainEpochClock, drand beacon.Schedule) *node.Node {
	builder := test.NewNodeBuilder(t).
		WithBuilderOpt(node.ChainClockConfigOption(chainClock)).
		WithGenesisInit(seed.GenesisInitFunc).
		WithBuilderOpt(node.MonkeyPatchSetProofTypeOption(constants.DevRegisteredSealProof))
	if drand != nil {
		builder = builder.WithBuilderOpt(node.DrandConfigOption(drand))
	}
	return builder.Build(ctx)
}

func initNodeGenesisMiner(ctx context.Context, t *testing.T, nd *node.Node, seed *node.ChainSeed, minerIdx int, presealPath string) (address.Address, address.Address, error) {
	seed.GiveKey(t, nd, minerIdx)
	miner, owner := seed.GiveMiner(t, nd, 0)

	//gen, err := nd.Chain().ChainReader.GetGenesisBlock(ctx)
	//require.NoError(t, err)

	c := nd.Repo.Config()
	err := nd.Repo.ReplaceConfig(c)
	require.NoError(t, err)

	return miner, owner, err
}
