package commands

import (
	"context"
	"encoding/json"
	"testing"

	cmds "gx/ipfs/QmYMj156vnPY7pYvtkvQiMDAzqWDDHkfiW5bYbMpYoHxhB/go-ipfs-cmds"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"

	asrt "github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestDagGetBadArg(t *testing.T) {
	assert := asrt.New(t)

	h := CreateInMemoryOfflineTestCmdHarness()

	env := &Env{node: h.node}

	_, err := testhelpers.RunCommand(dagGetCmd, []string{"awful"}, map[string]interface{}{
		cmds.EncLong: cmds.JSON,
	}, env)
	assert.Error(err)
}

func TestDagGetNoMatch(t *testing.T) {
	ctx := context.Background()
	assert := asrt.New(t)

	h := CreateInMemoryOfflineTestCmdHarness()

	env := &Env{node: h.node}

	err := h.node.ChainMgr.Genesis(ctx, core.InitGenesis)
	assert.NoError(err)

	cid := types.NewCidForTestGetter()()

	_, err = testhelpers.RunCommand(dagGetCmd, []string{cid.String()}, map[string]interface{}{
		cmds.EncLong: cmds.JSON,
	}, env)
	assert.Error(err)
	assert.Equal(ipld.ErrNotFound, err)
}

func TestDagGetMatch(t *testing.T) {
	ctx := context.Background()
	assert := asrt.New(t)

	h := CreateInMemoryOfflineTestCmdHarness()

	env := &Env{node: h.node}

	err := h.chainmanager.Genesis(ctx, core.InitGenesis)
	assert.NoError(err)

	gen := h.chainmanager.GetBestBlock()
	child := types.NewBlockForTest(gen, 1)
	h.chainmanager.ProcessNewBlock(ctx, child)

	out, err := testhelpers.RunCommand(dagGetCmd, []string{child.Cid().String()}, map[string]interface{}{
		cmds.EncLong: cmds.JSON,
	}, env)
	assert.NoError(err)

	var result types.Block
	err = json.Unmarshal([]byte(out.Raw), &result)
	assert.NoError(err)

	assert.True(result.Parent.Equals(gen.Cid()))
	assert.Equal(child.Nonce, result.Nonce)
}
