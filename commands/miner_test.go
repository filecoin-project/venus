package commands

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/testhelpers"
)

func TestMinerGenBlock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	assert := assert.New(t)

	nd, err := node.New(ctx)
	assert.NoError(err)
	defer nd.Stop()

	addr := nd.Wallet.NewAddress()

	out, err := testhelpers.RunCommand(minerGenBlockCmd, nil, nil, &Env{node: nd})
	assert.NoError(err)

	blk := nd.ChainMgr.GetBestBlock()
	assert.Equal(uint64(1), blk.Height)
	assert.NoError(out.HasLine(blk.Cid().String()))
	assert.Len(blk.Messages, 1)
	assert.Equal(addr, blk.Messages[0].To)
}
