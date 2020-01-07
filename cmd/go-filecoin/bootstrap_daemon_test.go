package commands_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestBootstrapList(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()
	builder := test.NewNodeBuilder(t)

	n := builder.BuildAndStart(ctx)
	defer n.Stop(ctx)
	cmdClient, done := test.RunNodeAPI(ctx, n, t)
	defer done()

	bs := cmdClient.RunSuccess(ctx, "bootstrap", "ls")

	assert.Equal(t, "&{[]}\n", bs.ReadStdout())
}
