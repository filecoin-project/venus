package node_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-state-types/abi"

	specsbig "github.com/filecoin-project/go-state-types/big"
	. "github.com/filecoin-project/venus/internal/app/go-filecoin/node"
	"github.com/filecoin-project/venus/internal/app/go-filecoin/node/test"
	"github.com/filecoin-project/venus/internal/pkg/constants"
	"github.com/filecoin-project/venus/internal/pkg/proofs"
	init_ "github.com/filecoin-project/venus/internal/pkg/specactors/builtin/init"
	th "github.com/filecoin-project/venus/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/internal/pkg/types"
	"github.com/filecoin-project/venus/internal/pkg/version"
	gengen "github.com/filecoin-project/venus/tools/gengen/util"
)

// TestMessagePropagation is a high level check that messages are propagated between message
// pools of connected nodes.
func TestMessagePropagation(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Generate a key and install an account actor at genesis which will be able to send messages.
	genCfg := &gengen.GenesisCfg{}
	require.NoError(t, gengen.GenKeys(1, "1000000")(genCfg))
	require.NoError(t, gengen.NetworkName(version.TEST)(genCfg))

	cs := MakeChainSeed(t, genCfg)

	// Initialize the first node to be the message sender.
	builder1 := test.NewNodeBuilder(t)
	builder1.WithGenesisInit(cs.GenesisInitFunc)
	builder1.WithBuilderOpt(VerifierConfigOption(&proofs.FakeVerifier{}))
	builder1.WithBuilderOpt(MonkeyPatchSetProofTypeOption(constants.DevRegisteredSealProof))

	sender := builder1.Build(ctx)
	senderAddress := cs.GiveKey(t, sender, 0)

	// Initialize other nodes to receive the message.
	builder2 := test.NewNodeBuilder(t)
	builder2.WithGenesisInit(cs.GenesisInitFunc)
	builder2.WithBuilderOpt(VerifierConfigOption(&proofs.FakeVerifier{}))
	builder2.WithBuilderOpt(MonkeyPatchSetProofTypeOption(constants.DevRegisteredSealProof))
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
			init_.Address,
			specsbig.NewInt(100),
			types.NewGasFeeCap(1),
			types.NewGasPremium(1),
			types.Unit(5000),
			fooMethod,
			[]byte{},
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
