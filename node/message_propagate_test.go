package node

import (
	"context"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"testing"
	"time"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestMessagePropagation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require := require.New(t)

	nodes := MakeNodesUnstarted(t, 5, false)
	StartNodes(t, nodes)
	defer StopNodes(nodes)
	connect(t, nodes[0], nodes[1])
	connect(t, nodes[1], nodes[2])
	connect(t, nodes[2], nodes[3])
	connect(t, nodes[3], nodes[4])

	// Wait for network connection notifications to propagate
	time.Sleep(time.Millisecond * 50)

	require.Equal(0, len(nodes[0].MsgPool.Pending()))
	require.Equal(0, len(nodes[1].MsgPool.Pending()))
	require.Equal(0, len(nodes[2].MsgPool.Pending()))
	require.Equal(0, len(nodes[3].MsgPool.Pending()))
	require.Equal(0, len(nodes[4].MsgPool.Pending()))

	nd0Addr, err := nodes[0].NewAddress()
	require.NoError(err)

	gasPrice := types.NewGasPrice(0)
	gasLimit := types.NewGasUnits(0)

	t.Run("Make sure new message makes it to every node message pool and is correctly propagated", func(t *testing.T) {
		_, err = nodes[0].PorcelainAPI.MessageSendWithDefaultAddress(
			ctx,
			nd0Addr,
			address.NetworkAddress,
			types.NewAttoFILFromFIL(123),
			gasPrice,
			gasLimit,
			"foo",
			[]byte{},
		)
		require.NoError(err)

		var msgs0, msgs1, msgs2, msgs3, msgs4 []*types.SignedMessage
		require.NoError(th.WaitForIt(50, 100*time.Millisecond, func() (bool, error) {
			msgs0 = nodes[0].MsgPool.Pending()
			msgs1 = nodes[1].MsgPool.Pending()
			msgs2 = nodes[2].MsgPool.Pending()
			msgs3 = nodes[3].MsgPool.Pending()
			msgs4 = nodes[4].MsgPool.Pending()
			return len(msgs0) == 1 && len(msgs1) == 1 && len(msgs2) == 1 && len(msgs3) == 1 && len(msgs4) == 1, nil
		}), "failed to propagate messages")

		assert.True(t, msgs0[0].Message.Method == "foo")
		assert.True(t, msgs4[0].Message.Method == "foo")
	})
}
