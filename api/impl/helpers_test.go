package impl

import (
	"testing"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/api2/impl/msg"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/stretchr/testify/assert"
)

func TestSetDefaultFromAddr(t *testing.T) {
	assert := assert.New(t)

	addr := address.Address{}
	nd := node.MakeOfflineNode(t)

	expected, err := msg.GetAndMaybeSetDefaultSenderAddress(nd.Repo, nd.Wallet)
	assert.NoError(err)
	assert.NotEqual(expected, address.Address{})

	assert.NoError(setDefaultFromAddr(&addr, nd))
	assert.Equal(expected, addr)
}
