package commands_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/venus/internal/app/go-filecoin/node"
	"github.com/filecoin-project/venus/internal/app/go-filecoin/node/test"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
)

func TestSwarmConnectPeersValid(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()
	builder := test.NewNodeBuilder(t)

	n1 := builder.BuildAndStart(ctx)
	defer n1.Stop(ctx)
	n2 := builder.BuildAndStart(ctx)
	defer n2.Stop(ctx)

	node.ConnectNodes(t, n1, n2)
}

func TestSwarmConnectPeersInvalid(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()
	builder := test.NewNodeBuilder(t)

	_, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	cmdClient.RunFail(ctx, "failed to parse ip4 addr",
		"swarm", "connect", "/ip4/hello",
	)
}
