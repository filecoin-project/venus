package node

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestMessagePropagation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require := require.New(t)

	nodes := MakeNodesUnstarted(t, 3, false, true)
	startNodes(t, nodes)
	defer stopNodes(nodes)
	connect(t, nodes[0], nodes[1])
	connect(t, nodes[1], nodes[2])

	// Wait for network connection notifications to propagate
	time.Sleep(time.Millisecond * 50)

	require.Equal(0, len(nodes[0].MsgPool.Pending()))
	require.Equal(0, len(nodes[1].MsgPool.Pending()))
	require.Equal(0, len(nodes[2].MsgPool.Pending()))

	nd0Addr, err := nodes[0].NewAddress()
	require.NoError(err)

	_, err = nodes[0].PlumbingAPI.MessageSend(ctx, nd0Addr, address.NetworkAddress, types.NewAttoFILFromFIL(123), "")
	require.NoError(err)

	// Wait for message to propagate across network
	require.NoError(th.WaitForIt(50, 100*time.Millisecond, func() (bool, error) {
		l1 := len(nodes[0].MsgPool.Pending())
		l2 := len(nodes[1].MsgPool.Pending())
		l3 := len(nodes[2].MsgPool.Pending())
		return l1 == 1 && l2 == 1 && l3 == 1, nil
	}), "failed to propagate messages")
}
