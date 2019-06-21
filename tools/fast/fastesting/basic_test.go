package fastesting_test

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-log"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/fastesting"
)

func TestReproduceFASTFailure(t *testing.T) {
	tf.IntegrationTest(t)
	log.SetDebugLogging()

	fastOpts := fast.FilecoinOpts{
		DaemonOpts: []fast.ProcessDaemonOption{fast.POBlockTime(time.Second)},
	}

	ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fastOpts)

	clientNode := env.GenesisMiner
	require.NoError(t, clientNode.MiningStart(ctx))
}
