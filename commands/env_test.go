package commands

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/core/node"
	"github.com/filecoin-project/go-filecoin/repo"
)

func TestEnv(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	ctx := context.Background()
	r := repo.NewInMemoryRepo()

	err := node.Init(ctx, r, core.InitGenesis)
	assert.NoError(err)

	opts, err := node.OptionsFromRepo(r)
	assert.NoError(err)

	nd, err := node.New(ctx, opts...)
	assert.NoError(err)

	env := Env{ctx: ctx, node: nd}

	assert.Equal(env.Node(), nd)
	assert.Equal(env.Context(), ctx)
	assert.Equal(GetNode(&env), nd)
}
