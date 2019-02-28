package node

import (
	"context"
	"testing"
	"time"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"
)

// TestMessagePropagation is a high level check that messages are propagated between message
// pools of connected ndoes.
func TestMessagePropagation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require := require.New(t)

	// Generate a key and install an account actor at genesis which will be able to send messages.
	ki := types.MustGenerateKeyInfo(1, types.GenerateKeyInfoSeed())[0]
	senderAddress, err := ki.Address()
	require.NoError(err)
	genesis := consensus.MakeGenesisFunc(
		consensus.ActorAccount(senderAddress, types.NewAttoFILFromFIL(100)),
	)

	// Initialize the first node to be the message sender.
	senderNodeOpts := TestNodeOptions{
		GenesisFunc: genesis,
		ConfigOpts:  DefaultTestingConfig(),
		InitOpts: []InitOpt{
			DefaultWalletAddressOpt(senderAddress),
		},
	}
	sender := GenNode(t, &senderNodeOpts)

	// Give the sender the private key so it can sign messages.
	// Note: this is an ugly hack around the Wallet lacking a mutable interface.
	err = sender.Wallet.Backends(wallet.DSBackendType)[0].(*wallet.DSBackend).ImportKey(&ki)
	require.NoError(err)

	// Initialize other nodes to receive the message.
	receiverCount := 2
	receivers := MakeNodesUnstartedWithGif(t, receiverCount, false, genesis)

	nodes := append([]*Node{sender}, receivers...)
	StartNodes(t, nodes)
	defer StopNodes(nodes)

	// Connect nodes in series
	connect(t, nodes[0], nodes[1])
	connect(t, nodes[1], nodes[2])
	// Wait for network connection notifications to propagate
	time.Sleep(time.Millisecond * 50)

	require.Equal(0, len(nodes[0].MsgPool.Pending()))
	require.Equal(0, len(nodes[1].MsgPool.Pending()))
	require.Equal(0, len(nodes[2].MsgPool.Pending()))

	t.Run("message propagates", func(t *testing.T) {
		_, err = sender.PorcelainAPI.MessageSendWithDefaultAddress(
			ctx,
			senderAddress,
			address.NetworkAddress,
			types.NewAttoFILFromFIL(1),
			types.NewGasPrice(0),
			types.NewGasUnits(0),
			"foo",
		)
		require.NoError(err)

		require.NoError(th.WaitForIt(50, 100*time.Millisecond, func() (bool, error) {
			return len(nodes[0].MsgPool.Pending()) == 1 &&
				len(nodes[1].MsgPool.Pending()) == 1 &&
				len(nodes[2].MsgPool.Pending()) == 1, nil
		}), "failed to propagate messages")

		assert.True(t, nodes[0].MsgPool.Pending()[0].Message.Method == "foo")
		assert.True(t, nodes[1].MsgPool.Pending()[0].Message.Method == "foo")
		assert.True(t, nodes[2].MsgPool.Pending()[0].Message.Method == "foo")
	})
}
