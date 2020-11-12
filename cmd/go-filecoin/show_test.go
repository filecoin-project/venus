package commands_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/vm"

	"github.com/filecoin-project/venus/fixtures/fortest"
	"github.com/filecoin-project/venus/internal/app/go-filecoin/node"
	"github.com/filecoin-project/venus/internal/app/go-filecoin/node/test"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/internal/pkg/types"
)

func TestBlockDaemon(t *testing.T) {
	tf.IntegrationTest(t)
	t.Skip("Unskip with fake proofs")

	t.Run("show block <cid-of-genesis-block> returns human readable output for the filecoin block", func(t *testing.T) {
		ctx := context.Background()
		builder := test.NewNodeBuilder(t)
		buildWithMiner(t, builder)

		n, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		// mine a block and get its CID
		minedBlock, err := n.BlockMining.BlockMiningAPI.MiningOnce(ctx)
		require.NoError(t, err)
		minedBlockCid := minedBlock.Cid()

		// get the mined block by its CID
		output := cmdClient.RunSuccess(ctx, "show", "block", minedBlockCid.String()).ReadStdoutTrimNewlines()

		assert.Contains(t, output, "Block Details")
		assert.Contains(t, output, "Weight: 0")
		assert.Contains(t, output, "Height: 1")
		assert.Contains(t, output, "Timestamp:  ")
	})

	t.Run("show block --messages <cid-of-genesis-block> returns human readable output for the filecoin block including messages", func(t *testing.T) {
		ctx := context.Background()
		builder := test.NewNodeBuilder(t)
		buildWithMiner(t, builder)

		n, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		// mine a block and get its CID
		minedBlock, err := n.BlockMining.BlockMiningAPI.MiningOnce(ctx)
		require.NoError(t, err)

		// get the mined block by its CID
		output := cmdClient.RunSuccess(ctx, "show", "block", "--messages", minedBlock.Cid().String()).ReadStdoutTrimNewlines()

		assert.Contains(t, output, "Block Details")
		assert.Contains(t, output, "Weight: 0")
		assert.Contains(t, output, "Height: 1")
		assert.Contains(t, output, "Timestamp:  ")
		assert.Contains(t, output, "Messages:  ")
	})

	t.Run("show block <cid-of-genesis-block> --enc json returns JSON for a filecoin block", func(t *testing.T) {
		ctx := context.Background()
		builder := test.NewNodeBuilder(t)
		buildWithMiner(t, builder)

		n, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		// mine a block and get its CID
		minedBlock, err := n.BlockMining.BlockMiningAPI.MiningOnce(ctx)
		require.NoError(t, err)

		// get the mined block by its CID
		blockGetLine := cmdClient.RunSuccessFirstLine(ctx, "show", "block", minedBlock.Cid().String(), "--enc", "json")
		var blockGetBlock block.FullBlock
		require.NoError(t, json.Unmarshal([]byte(blockGetLine), &blockGetBlock))

		// ensure that we were returned the correct block

		require.Equal(t, minedBlock.Cid().String(), blockGetBlock.Header.Cid().String())
	})

	t.Run("show header <cid-of-genesis-block> --enc json returns JSON for a filecoin block header", func(t *testing.T) {
		ctx := context.Background()
		builder := test.NewNodeBuilder(t)
		buildWithMiner(t, builder)

		n, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		// mine a block and get its CID
		minedBlock, err := n.BlockMining.BlockMiningAPI.MiningOnce(ctx)
		require.NoError(t, err)

		// get the mined block by its CID
		headerGetLine := cmdClient.RunSuccessFirstLine(ctx, "show", "header", minedBlock.Cid().String(), "--enc", "json")

		var headerGetBlock block.Block
		require.NoError(t, json.Unmarshal([]byte(headerGetLine), &headerGetBlock))

		// ensure that we were returned the correct block

		require.Equal(t, minedBlock.Cid().String(), headerGetBlock.Cid().String())
	})

	t.Run("show messages <empty-collection-cid> returns empty message collection", func(t *testing.T) {
		ctx := context.Background()
		builder := test.NewNodeBuilder(t)
		buildWithMiner(t, builder)

		n, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		_, err := n.BlockMining.BlockMiningAPI.MiningOnce(ctx)
		require.NoError(t, err)

		emptyMessagesLine := cmdClient.RunSuccessFirstLine(ctx, "show", "messages", types.EmptyMessagesCID.String(), "--enc", "json")

		var messageCollection []*types.SignedMessage
		require.NoError(t, json.Unmarshal([]byte(emptyMessagesLine), &messageCollection))

		assert.Equal(t, 0, len(messageCollection))
	})

	t.Run("show receipts <empty-collection-cid> returns empty receipt collection", func(t *testing.T) {
		ctx := context.Background()
		builder := test.NewNodeBuilder(t)
		buildWithMiner(t, builder)

		n, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		_, err := n.BlockMining.BlockMiningAPI.MiningOnce(ctx)
		require.NoError(t, err)

		emptyReceiptsLine := cmdClient.RunSuccessFirstLine(ctx, "show", "receipts", types.EmptyReceiptsCID.String(), "--enc", "json")

		var receipts []vm.MessageReceipt
		require.NoError(t, json.Unmarshal([]byte(emptyReceiptsLine), &receipts))

		assert.Equal(t, 0, len(receipts))
	})

	t.Run("show messages", func(t *testing.T) {
		cs := node.FixtureChainSeed(t)
		defaultAddr := fortest.TestAddresses[0]
		ctx := context.Background()
		builder := test.NewNodeBuilder(t)
		builder.WithGenesisInit(cs.GenesisInitFunc)
		builder.WithConfig(cs.MinerConfigOpt(0))
		builder.WithConfig(node.DefaultAddressConfigOpt(defaultAddr))
		builder.WithInitOpt(cs.KeyInitOpt(1))
		builder.WithInitOpt(cs.KeyInitOpt(0))

		n, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		_, err := n.BlockMining.BlockMiningAPI.MiningOnce(ctx)
		require.NoError(t, err)

		from, err := n.PorcelainAPI.WalletDefaultAddress() // this should = fixtures.TestAddresses[0]
		require.NoError(t, err)
		cmdClient.RunSuccess(ctx, "message", "send",
			"--from", from.String(),
			"--gas-price", "1",
			"--gas-limit", "300",
			fortest.TestAddresses[3].String(),
		)

		cmdClient.RunSuccess(ctx, "message", "send",
			"--from", from.String(),
			"--gas-price", "1",
			"--gas-limit", "300",
			"--value", "10",
			fortest.TestAddresses[3].String(),
		)

		cmdClient.RunSuccess(ctx, "message", "send",
			"--from", from.String(),
			"--gas-price", "1",
			"--gas-limit", "300",
			"--value", "5.5",
			fortest.TestAddresses[3].String(),
		)

		blk, err := n.BlockMining.BlockMiningAPI.MiningOnce(ctx)
		require.NoError(t, err)

		// Full block checks out
		blockGetLine := cmdClient.RunSuccessFirstLine(ctx, "show", "block", blk.Cid().String(), "--enc", "json")
		var blockGetBlock block.FullBlock
		require.NoError(t, json.Unmarshal([]byte(blockGetLine), &blockGetBlock))

		assert.Equal(t, 3, len(blockGetBlock.SECPMessages))

		assert.Equal(t, from, blockGetBlock.SECPMessages[0].Message.From)

		// Full block matches show messages
		messagesGetLine := cmdClient.RunSuccessFirstLine(ctx, "show", "messages", blockGetBlock.Header.Messages.String(), "--enc", "json")
		var messages []*types.SignedMessage
		require.NoError(t, json.Unmarshal([]byte(messagesGetLine), &messages))
		assert.Equal(t, blockGetBlock.SECPMessages, messages)
	})
}
