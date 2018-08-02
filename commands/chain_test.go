package commands

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChainHead(t *testing.T) {
	t.Parallel()
	t.Run("returns an error if no best block", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)

		n := node.MakeNodesUnstarted(t, 1, true, true)[0]

		res, err := testhelpers.RunCommandJSONEnc(chainHeadCmd, []string{}, nil, &Env{
			ctx:  context.Background(),
			node: n,
		})
		require.NoError(err)
		require.Contains(res.Raw, ErrHeaviestTipSetNotFound.Error())
	})

	t.Run("emits the blockchain head", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)
		assert := assert.New(t)

		ctx := context.Background()
		blk := types.NewBlockForTest(nil, 1)
		n := node.MakeNodesUnstarted(t, 1, true, true)[0]

		n.ChainMgr.SetHeaviestTipSetForTest(ctx, core.RequireNewTipSet(require, blk))

		out, err := testhelpers.RunCommandJSONEnc(chainHeadCmd, []string{}, nil, &Env{
			ctx:  ctx,
			node: n,
		})
		require.NoError(err)

		assert.Contains(out.Raw, blk.Cid().String())
	})
}

func TestChainLsRun(t *testing.T) {
	t.Parallel()
	t.Run("chain of height two", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)
		assert := assert.New(t)

		ctx := context.Background()
		n := node.MakeNodesUnstarted(t, 1, true, true)[0]

		err := n.ChainMgr.Genesis(ctx, core.InitGenesis)
		require.NoError(err)
		genBlock := core.RequireBestBlock(n.ChainMgr, t)
		chlBlock := types.NewBlockForTest(genBlock, 1)

		err = n.ChainMgr.SetHeaviestTipSetForTest(ctx, core.RequireNewTipSet(require, chlBlock))
		require.NoError(err)

		out, err := testhelpers.RunCommandJSONEnc(chainLsCmd, []string{}, nil, &Env{
			ctx:  ctx,
			node: n,
		})
		require.NoError(err)

		lines := strings.Split(strings.Trim(out.Raw, "\n"), "\n")

		var bs [][]types.Block
		for _, line := range lines {
			var b []types.Block
			err := json.Unmarshal([]byte(line), &b)
			require.NoError(err)
			bs = append(bs, b)
		}

		assert.Equal(2, len(bs))
		types.AssertHaveSameCid(assert, chlBlock, &bs[0][0])
		types.AssertHaveSameCid(assert, genBlock, &bs[1][0])
	})

	t.Run("emit best block and then time out getting parent", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)

		ctx := context.Background()
		n := node.MakeNodesUnstarted(t, 1, true, true)[0]

		parBlock := types.NewBlockForTest(nil, 0)
		chlBlock := types.NewBlockForTest(parBlock, 1)

		err := n.ChainMgr.SetHeaviestTipSetForTest(ctx, core.RequireNewTipSet(require, chlBlock))
		require.NoError(err)

		// parBlock is not known to the chain, which causes the timeout
		res, err := testhelpers.RunCommandJSONEnc(chainLsCmd, []string{}, nil, &Env{
			ctx:  ctx,
			node: n,
		})
		require.NoError(err)
		require.Contains(res.Raw, "error fetching block")
	})

	t.Run("JSON marshaling", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		parent := types.NewBlockForTest(nil, 0)
		child := types.NewBlockForTest(parent, 1)

		// Generate a single private/public key pair
		ki := types.MustGenerateKeyInfo(1, types.GenerateKeyInfoSeed())
		// Create a mockSigner (bad name) that can sign using the previously generated key
		mockSigner := types.NewMockSigner(ki)
		// Generate SignedMessages
		newSignedMessage := types.NewSignedMessageForTestGetter(mockSigner)
		message := newSignedMessage()

		retVal := []byte{1, 2, 3}
		receipt := &types.MessageReceipt{
			ExitCode: 123,
			Return:   []types.Bytes{retVal},
		}
		child.Messages = []*types.SignedMessage{message}
		child.MessageReceipts = []*types.MessageReceipt{receipt}

		marshaled, e1 := json.Marshal(child)
		assert.NoError(e1)
		str := string(marshaled)

		assert.Contains(str, parent.Cid().String())
		assert.Contains(str, message.From.String())
		assert.Contains(str, message.To.String())

		// marshal/unmarshal symmetry
		var unmarshalled types.Block
		e2 := json.Unmarshal(marshaled, &unmarshalled)
		assert.NoError(e2)

		assert.Equal(uint8(123), unmarshalled.MessageReceipts[0].ExitCode)
		assert.Equal([]types.Bytes{[]byte{1, 2, 3}}, unmarshalled.MessageReceipts[0].Return)

		types.AssertHaveSameCid(assert, child, &unmarshalled)
	})
}
