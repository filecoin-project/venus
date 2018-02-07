package commands

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/testhelpers"
)

func TestAddrsNew(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	nd, err := node.New(ctx)
	assert.NoError(err)

	out, err := testhelpers.RunCommand(addrsNewCmd, nil, nil, &Env{node: nd})
	assert.NoError(err)

	assert.NoError(out.HasLine(nd.Wallet.GetAddresses()[0].String()))
}

func TestAddrsList(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	nd, err := node.New(ctx)
	assert.NoError(err)

	a1 := nd.Wallet.NewAddress()
	a2 := nd.Wallet.NewAddress()
	a3 := nd.Wallet.NewAddress()

	out, err := testhelpers.RunCommand(addrsListCmd, nil, nil, &Env{node: nd})
	assert.NoError(err)

	assert.NoError(out.HasLine(a1.String()))
	assert.NoError(out.HasLine(a2.String()))
	assert.NoError(out.HasLine(a3.String()))
}
