package node_test

import (
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/specs-actors/actors/abi"

	. "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	"github.com/filecoin-project/go-filecoin/internal/pkg/version"

	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// TestMessagePropagation is a high level check that messages are propagated between message
// pools of connected ndoes.
func TestMessagePropagation(t *testing.T) {
	tf.UnitTest(t)
	t.Skip("its using the old vmcontext")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Generate a key and install an account actor at genesis which will be able to send messages.
	ki := types.MustGenerateKeyInfo(1, 42)[0]
	senderAddress, err := ki.Address()
	require.NoError(t, err)
	genesis := consensus.MakeGenesisFunc(
		consensus.ActorAccount(senderAddress, abi.NewTokenAmount(100)),
		consensus.Network(version.TEST),
	)

	// Initialize the first node to be the message sender.
	builder1 := test.NewNodeBuilder(t)
	builder1.WithGenesisInit(genesis)
	builder1.WithInitOpt(DefaultKeyOpt(&ki))
	builder1.WithBuilderOpt(FakeProofVerifierBuilderOpts()...)

	sender := builder1.Build(ctx)

	// Initialize other nodes to receive the message.
	builder2 := test.NewNodeBuilder(t)
	builder2.WithGenesisInit(genesis)
	builder2.WithBuilderOpt(FakeProofVerifierBuilderOpts()...)
	receiverCount := 2
	receivers := builder2.BuildMany(ctx, receiverCount)

	nodes := append([]*Node{sender}, receivers...)
	StartNodes(t, nodes)
	defer StopNodes(nodes)

	// Connect nodes in series
	ConnectNodes(t, nodes[0], nodes[1])
	ConnectNodes(t, nodes[1], nodes[2])
	// Wait for network connection notifications to propagate
	time.Sleep(time.Millisecond * 200)

	require.Equal(t, 0, len(nodes[1].Messaging.Inbox.Pool().Pending()))
	require.Equal(t, 0, len(nodes[2].Messaging.Inbox.Pool().Pending()))
	require.Equal(t, 0, len(nodes[0].Messaging.Inbox.Pool().Pending()))

	fooMethod := abi.MethodNum(7232)

	t.Run("message propagates", func(t *testing.T) {
		_, _, err := sender.PorcelainAPI.MessageSend(
			ctx,
			senderAddress,
			builtin.InitActorAddr,
			types.NewAttoFILFromFIL(1),
			types.NewGasPrice(1),
			types.GasUnits(0),
			fooMethod,
			&adt.EmptyValue{},
		)
		require.NoError(t, err)

		require.NoError(t, th.WaitForIt(50, 100*time.Millisecond, func() (bool, error) {
			return len(nodes[0].Messaging.Inbox.Pool().Pending()) == 1 &&
				len(nodes[1].Messaging.Inbox.Pool().Pending()) == 1 &&
				len(nodes[2].Messaging.Inbox.Pool().Pending()) == 1, nil
		}), "failed to propagate messages")

		assert.True(t, nodes[0].Messaging.Inbox.Pool().Pending()[0].Message.Method == fooMethod)
		assert.True(t, nodes[1].Messaging.Inbox.Pool().Pending()[0].Message.Method == fooMethod)
		assert.True(t, nodes[2].Messaging.Inbox.Pool().Pending()[0].Message.Method == fooMethod)
	})
}
