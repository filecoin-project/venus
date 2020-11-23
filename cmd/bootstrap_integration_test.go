package cmd_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/venus/app/node/test"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func TestBootstrapList(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()
	builder := test.NewNodeBuilder(t)

	_, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	bs := cmdClient.RunSuccess(ctx, "bootstrap", "ls")

	assert.Equal(t, "{\n\t\"Peers\": []\n}\n", bs.ReadStdout())
}
