package commands

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/node"
)

func TestEnv(t *testing.T) {
	assert := assert.New(t)

	ctx := context.Background()
	node, err := node.New(ctx)
	assert.NoError(err)

	env := Env{ctx: ctx, node: node}

	assert.Equal(env.Node(), node)
	assert.Equal(env.Context(), ctx)
	assert.Equal(GetNode(&env), node)
}
