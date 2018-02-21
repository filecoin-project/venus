package commands

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/testhelpers"
)

type msi map[string]interface{}

func TestMessageSend(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	assert := assert.New(t)

	nd, err := node.New(ctx)
	assert.NoError(err)
	defer nd.Stop()

	out, err := testhelpers.RunCommand(sendMsgCmd, []string{"0xf00ba4"}, msi{"value": 100, "from": "0x123456"}, &Env{node: nd})
	assert.NoError(err)

	pending := nd.MsgPool.Pending()
	assert.Len(pending, 1)
	assert.Equal(pending[0].From.String(), "0x123456")

	c, err := pending[0].Cid()
	assert.NoError(err)
	assert.NoError(out.HasLine(c.String()))
}
