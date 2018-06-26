package node

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestMessagePropagation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require := require.New(t)

	nodes := MakeNodesUnstarted(t, 3, false)
	startNodes(t, nodes)
	defer stopNodes(nodes)
	connect(t, nodes[0], nodes[1])
	connect(t, nodes[1], nodes[2])

	// Wait for network connection notifications to propagate
	time.Sleep(time.Millisecond * 50)

	require.Equal(0, len(nodes[0].MsgPool.Pending()))
	require.Equal(0, len(nodes[1].MsgPool.Pending()))
	require.Equal(0, len(nodes[2].MsgPool.Pending()))

	msg := types.NewMessage(address.NetworkAddress, address.TestAddress, 0, types.NewAttoFILFromFIL(123), "", nil)
	nodes[0].AddNewMessage(ctx, msg)

	// Wait for message to propagate across network
	time.Sleep(time.Millisecond * 50)

	require.Equal(1, len(nodes[0].MsgPool.Pending()))
	require.Equal(1, len(nodes[1].MsgPool.Pending()))
	require.Equal(1, len(nodes[2].MsgPool.Pending()))
}
