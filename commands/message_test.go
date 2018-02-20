package commands

import (
	"context"
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
)

type msi map[string]interface{}

func TestMessageSend(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	assert := assert.New(t)

	nd, err := node.New(ctx)
	assert.NoError(err)
	defer nd.Stop()

	out, err := testhelpers.RunCommand(sendMsgCmd, []string{"0xf00ba4", "balance"}, msi{"from": "0x123456", "offchain": false}, &Env{node: nd})
	assert.NoError(err)

	pending := nd.MsgPool.Pending()
	assert.Len(pending, 1)
	assert.Equal(pending[0].From.String(), "0x123456")

	c, err := pending[0].Cid()
	assert.NoError(err)
	assert.Contains(out.Raw, c.String())
}

func TestMessageSendOffChain(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	assert := assert.New(t)

	nd, err := node.New(ctx)
	assert.NoError(err)
	defer nd.Stop()

	fromAddr := types.Address("investor1")
	toAddr := types.Address("token")

	// TODO: fix me, need a way to send typed params to an actor.
	out, err := testhelpers.RunCommand(sendMsgCmd, []string{toAddr.String(), "balance", toAddr.String()}, msi{"from": fromAddr.String(), "offchain": true}, &Env{node: nd})
	assert.NoError(err)

	assert.Len(nd.MsgPool.Pending(), 0)

	// the balance is currently encoded as big.Int.Bytes -> json
	val, err := json.Marshal(big.NewInt(100000).Bytes())
	assert.NoError(err)
	assert.Contains(out.Raw, strings.Trim(string(val), `"`))
}
