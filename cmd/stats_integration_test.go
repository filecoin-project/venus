package cmd_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/venus/app/node/test"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func TestStatsBandwidth(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()
	builder := test.NewNodeBuilder(t)

	_, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	stats := cmdClient.RunSuccess(ctx, "stats", "bandwidth").ReadStdoutTrimNewlines()

	assert.Equal(t, "{\n\t\"TotalIn\": 0,\n\t\"TotalOut\": 0,\n\t\"RateIn\": 0,\n\t\"RateOut\": 0\n}", stats)
}
