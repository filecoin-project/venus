package commands

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/api/impl"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/repo"
)

func TestEnv(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	ctx := context.Background()
	r := repo.NewInMemoryRepo()
	r.Config().Swarm.Address = "/ip4/0.0.0.0/tcp/0"

	err := node.Init(ctx, r, consensus.InitGenesis)
	assert.NoError(err)

	opts, err := node.OptionsFromRepo(r)
	assert.NoError(err)

	nd, err := node.New(ctx, opts...)
	assert.NoError(err)

	api := impl.New(nd)

	env := Env{ctx: ctx, api: api}

	assert.Equal(env.API(), api)
	assert.Equal(env.Context(), ctx)
	assert.Equal(GetAPI(&env), api)
}
