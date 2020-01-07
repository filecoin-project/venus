package commands_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestStatsBandwidth(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()
	builder := test.NewNodeBuilder(t)

	_, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	stats := cmdClient.RunSuccess(ctx, "stats", "bandwidth").ReadStdoutTrimNewlines()

	assert.Equal(t, "{\"TotalIn\":0,\"TotalOut\":0,\"RateIn\":0,\"RateOut\":0}", stats)
}
