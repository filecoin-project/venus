package commands

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/testhelpers"
)

func TestId(t *testing.T) {
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	nd, err := node.New(ctx)
	assert.NoError(err)
	defer nd.Stop()

	out, err := testhelpers.RunCommand(idCmd, nil, &Env{node: nd})
	assert.NoError(err)

	assert.True(out.Contains(nd.Host.ID().Pretty()))
}
