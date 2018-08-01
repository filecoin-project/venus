package commands

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/core/node"
	"github.com/filecoin-project/go-filecoin/node/impl"
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

	api := impl.NewCoreAPI(nd)

	env := Env{ctx: ctx, api: api}

	assert.Equal(env.API(), api)
	assert.Equal(env.Context(), ctx)
	assert.Equal(GetAPI(&env), api)
}
