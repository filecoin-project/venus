package functional

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/drand"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDrandPublic(t *testing.T) {
	tf.FunctionalTest(t)
	t.Skip(("requires local drand setup"))

	ctx := context.Background()
	wd, _ := os.Getwd()
	genCfgPath := filepath.Join(wd, "..", "fixtures/setup.json")
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

	nd := makeNode(ctx, t, seed, chainClock)

	err := nd.Start(ctx)
	require.NoError(t, err)
	defer nd.Stop(ctx)

	err = nd.DrandAPI.Configure([]string{
		"localhost:8080",
		"localhost:8081",
		"localhost:8082",
		"localhost:8083",
		"localhost:8084"}, false, true)
	require.NoError(t, err)

	entry, err := nd.DrandAPI.GetEntry(ctx, 1)
	require.NoError(t, err)

	assert.Equal(t, drand.Round(1), entry.Round)
	assert.NotNil(t, entry.Signature)
}
