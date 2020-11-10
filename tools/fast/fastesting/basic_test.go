package fastesting_test

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-log/v2"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/tools/fast"
	"github.com/filecoin-project/venus/tools/fast/fastesting"
)

func TestSetFilecoinOpts(t *testing.T) {
	tf.IntegrationTest(t)
	t.Skip("not working")
	log.SetDebugLogging()

	fastOpts := fast.FilecoinOpts{
		DaemonOpts: []fast.ProcessDaemonOption{fast.POBlockTime(10 * time.Millisecond)},
	}

	ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fastOpts)
	clientNode := env.GenesisMiner
	require.NoError(t, clientNode.MiningStart(ctx))
	defer func() {
		err := env.Teardown(ctx)
		require.NoError(t, err)
	}()
}

func TestNoFilecoinOpts(t *testing.T) {
	tf.IntegrationTest(t)
	t.Skip("not working")
	log.SetDebugLogging()

	ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fast.FilecoinOpts{})

	clientNode := env.GenesisMiner
	require.NoError(t, clientNode.MiningStart(ctx))
	defer func() {
		err := env.Teardown(ctx)
		require.NoError(t, err)
	}()
}
