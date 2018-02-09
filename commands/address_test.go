package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestAddrsNew(t *testing.T) {
	assert := assert.New(t)

	nd := &node.Node{Wallet: types.NewWallet()}

	out, err := testhelpers.RunCommand(addrsNewCmd, nil, &Env{node: nd})
	assert.NoError(err)

	assert.True(out.HasLine(nd.Wallet.GetAddresses()[0].String()))
}

func TestAddrsList(t *testing.T) {
	assert := assert.New(t)

	nd := &node.Node{Wallet: types.NewWallet()}
	a1 := nd.Wallet.NewAddress()
	a2 := nd.Wallet.NewAddress()
	a3 := nd.Wallet.NewAddress()

	out, err := testhelpers.RunCommand(addrsListCmd, nil, &Env{node: nd})
	assert.NoError(err)

	assert.True(out.HasLine(a1.String()))
	assert.True(out.HasLine(a2.String()))
	assert.True(out.HasLine(a3.String()))
}
