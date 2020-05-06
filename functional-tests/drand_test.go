package functional

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/drand"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestDrandPublic(t *testing.T) {
	tf.FunctionalTest(t)
	t.Skip(("requires local drand setup"))

	ctx := context.Background()
	genTime := int64(1000000000)
	blockTime := 30 * time.Second
	// The clock is intentionally set some way ahead of the genesis time so the miner can produce
	// catch-up blocks as quickly as possible.
	fakeClock := clock.NewFake(time.Unix(genTime, 0).Add(4 * time.Hour))

	// The fixture is needed in order to use the presealed genesis sectors fixture.
	// Future code could decouple the whole setup.json from the presealed information.
	genCfg := loadGenesisConfig(t, fixtureGenCfg())
	seed := node.MakeChainSeed(t, genCfg)
	chainClock := clock.NewChainClockFromClock(uint64(genTime), blockTime, fakeClock)

	nd := makeNode(ctx, t, seed, chainClock, nil)

	err := nd.Start(ctx)
	require.NoError(t, err)
	defer nd.Stop(ctx)

	err = nd.DrandAPI.Configure([]string{
		"drand-test3.nikkolasg.xyz:5003",
	}, true, false)
	require.NoError(t, err)

	entry1, err := nd.DrandAPI.GetEntry(ctx, 1)
	require.NoError(t, err)

	assert.Equal(t, drand.Round(1), entry1.Round)
	assert.NotNil(t, entry1.Data)

	entry2, err := nd.DrandAPI.GetEntry(ctx, 2)
	require.NoError(t, err)

	valid, err := nd.DrandAPI.VerifyEntry(entry1, entry2)
	require.NoError(t, err)
	require.True(t, valid)
}
