package commands_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestPing2Nodes(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()
	builder := test.NewNodeBuilder(t)

	n1 := builder.BuildAndStart(ctx)
	n2 := builder.BuildAndStart(ctx)
	defer n1.Stop(ctx)
	defer n2.Stop(ctx)

	// The node are not initially connected, so ping should fail.
	res0, err := n1.PorcelainAPI.NetworkPing(ctx, n2.Network().Host.ID())
	assert.NoError(t, err)
	assert.Error(t, (<-res0).Error) // No peers in table

	// Connect nodes and check each can ping the other.
	node.ConnectNodes(t, n1, n2)

	res1, err := n1.PorcelainAPI.NetworkPing(ctx, n2.Network().Host.ID())
	assert.NoError(t, err)
	assert.NoError(t, (<-res1).Error)

	res2, err := n2.PorcelainAPI.NetworkPing(ctx, n1.Network().Host.ID())
	assert.NoError(t, err)
	assert.NoError(t, (<-res2).Error)
}
