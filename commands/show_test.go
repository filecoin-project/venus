package commands_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/fixtures"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestBlockDaemon(t *testing.T) {
	tf.IntegrationTest(t)

	t.Run("show block <cid-of-genesis-block> returns human readable output for the filecoin block", func(t *testing.T) {
		d := makeTestDaemonWithMinerAndStart(t)
		defer d.ShutdownSuccess()

		// mine a block and get its CID
		minedBlockCidStr := th.RunSuccessFirstLine(d, "mining", "once")

		// get the mined block by its CID
		output := d.RunSuccess("show", "block", minedBlockCidStr).ReadStdoutTrimNewlines()

		assert.Contains(t, output, "Block Details")
		assert.Contains(t, output, "Weight: 0")
		assert.Contains(t, output, "Height: 1")
		assert.Contains(t, output, "Nonce:  0")
		assert.Contains(t, output, "Timestamp:  ")
	})

	t.Run("show block --messages <cid-of-genesis-block> returns human readable output for the filecoin block including messages", func(t *testing.T) {
		d := makeTestDaemonWithMinerAndStart(t)
		defer d.ShutdownSuccess()

		// mine a block and get its CID
		minedBlockCidStr := th.RunSuccessFirstLine(d, "mining", "once")

		// get the mined block by its CID
		output := d.RunSuccess("show", "block", "--messages", minedBlockCidStr).ReadStdoutTrimNewlines()

		assert.Contains(t, output, "Block Details")
		assert.Contains(t, output, "Weight: 0")
		assert.Contains(t, output, "Height: 1")
		assert.Contains(t, output, "Nonce:  0")
		assert.Contains(t, output, "Timestamp:  ")
		assert.Contains(t, output, "Messages:  ")
	})

	t.Run("show block <cid-of-genesis-block> --enc json returns JSON for a filecoin block", func(t *testing.T) {
		d := th.NewDaemon(t,
			th.KeyFile(fixtures.KeyFilePaths()[0]),
			th.WithMiner(fixtures.TestMiners[0])).Start()
		defer d.ShutdownSuccess()

		// mine a block and get its CID
		minedBlockCidStr := th.RunSuccessFirstLine(d, "mining", "once")

		// get the mined block by its CID
		blockGetLine := th.RunSuccessFirstLine(d, "show", "block", minedBlockCidStr, "--enc", "json")
		var blockGetBlock types.FullBlock
		require.NoError(t, json.Unmarshal([]byte(blockGetLine), &blockGetBlock))

		// ensure that we were returned the correct block

		require.Equal(t, minedBlockCidStr, blockGetBlock.Header.Cid().String())

		// ensure that the JSON we received from block get conforms to schema

	})

	t.Run("show header <cid-of-genesis-block> --enc json returns JSON for a filecoin block header", func(t *testing.T) {
		d := th.NewDaemon(t,
			th.KeyFile(fixtures.KeyFilePaths()[0]),
			th.WithMiner(fixtures.TestMiners[0])).Start()
		defer d.ShutdownSuccess()

		// mine a block and get its CID
		minedBlockCidStr := th.RunSuccessFirstLine(d, "mining", "once")

		// get the mined block by its CID
		headerGetLine := th.RunSuccessFirstLine(d, "show", "header", minedBlockCidStr, "--enc", "json")

		var headerGetBlock types.Block
		require.NoError(t, json.Unmarshal([]byte(headerGetLine), &headerGetBlock))

		// ensure that we were returned the correct block

		require.Equal(t, minedBlockCidStr, headerGetBlock.Cid().String())

		// ensure that the JSON we received from block get conforms to schema

		requireSchemaConformance(t, []byte(headerGetLine), "filecoin_block")
	})

	t.Run("show messages <empty-collection-cid> returns empty message collection", func(t *testing.T) {
		d := th.NewDaemon(t,
			th.KeyFile(fixtures.KeyFilePaths()[0]),
			th.WithMiner(fixtures.TestMiners[0])).Start()
		defer d.ShutdownSuccess()

		// mine a block
		th.RunSuccessFirstLine(d, "mining", "once")

		emptyMessagesLine := th.RunSuccessFirstLine(d, "show", "messages", types.EmptyMessagesCID.String(), "--enc", "json")

		var messageCollection []*types.SignedMessage
		require.NoError(t, json.Unmarshal([]byte(emptyMessagesLine), &messageCollection))

		assert.Equal(t, 0, len(messageCollection))
	})

	t.Run("show receipts <empty-collection-cid> returns empty receipt collection", func(t *testing.T) {
		d := th.NewDaemon(t,
			th.KeyFile(fixtures.KeyFilePaths()[0]),
			th.WithMiner(fixtures.TestMiners[0])).Start()
		defer d.ShutdownSuccess()

		// mine a block
		th.RunSuccessFirstLine(d, "mining", "once")

		emptyReceiptsLine := th.RunSuccessFirstLine(d, "show", "receipts", types.EmptyReceiptsCID.String(), "--enc", "json")

		var receipts []*types.MessageReceipt
		require.NoError(t, json.Unmarshal([]byte(emptyReceiptsLine), &receipts))

		assert.Equal(t, 0, len(receipts))
	})

	t.Run("show messages and show receipts", func(t *testing.T) {
		d := th.NewDaemon(
			t,
			th.DefaultAddress(fixtures.TestAddresses[0]),
			th.KeyFile(fixtures.KeyFilePaths()[1]),
			// must include same-index KeyFilePath when configuring with a TestMiner.
			th.WithMiner(fixtures.TestMiners[0]),
			th.KeyFile(fixtures.KeyFilePaths()[0]),
		).Start()
		defer d.ShutdownSuccess()

		d.RunSuccess("mining", "once")

		from := d.GetDefaultAddress() // this should = fixtures.TestAddresses[0]

		d.RunSuccess("message", "send",
			"--from", from,
			"--gas-price", "1",
			"--gas-limit", "300",
			fixtures.TestAddresses[3],
		)

		d.RunSuccess("message", "send",
			"--from", from,
			"--gas-price", "1",
			"--gas-limit", "300",
			"--value", "10",
			fixtures.TestAddresses[3],
		)

		d.RunSuccess("message", "send",
			"--from", from,
			"--gas-price", "1",
			"--gas-limit", "300",
			"--value", "5.5",
			fixtures.TestAddresses[3],
		)

		minedBlockCidStr := th.RunSuccessFirstLine(d, "mining", "once")

		// Full block checks out
		blockGetLine := th.RunSuccessFirstLine(d, "show", "block", minedBlockCidStr, "--enc", "json")
		var blockGetBlock types.FullBlock
		require.NoError(t, json.Unmarshal([]byte(blockGetLine), &blockGetBlock))

		assert.Equal(t, 3, len(blockGetBlock.Messages))
		assert.Equal(t, 3, len(blockGetBlock.Receipts))

		fromAddr, err := address.NewFromString(from)
		require.NoError(t, err)
		assert.Equal(t, fromAddr, blockGetBlock.Messages[0].MeteredMessage.Message.From)
		assert.Equal(t, uint8(0), blockGetBlock.Receipts[0].ExitCode)

		// Full block matches show messages
		messagesGetLine := th.RunSuccessFirstLine(d, "show", "messages", blockGetBlock.Header.Messages.String(), "--enc", "json")
		var messages []*types.SignedMessage
		require.NoError(t, json.Unmarshal([]byte(messagesGetLine), &messages))
		assert.Equal(t, blockGetBlock.Messages, messages)

		// Full block matches show receipts
		receiptsGetLine := th.RunSuccessFirstLine(d, "show", "receipts", blockGetBlock.Header.MessageReceipts.String(), "--enc", "json")
		var receipts []*types.MessageReceipt
		require.NoError(t, json.Unmarshal([]byte(receiptsGetLine), &receipts))
		assert.Equal(t, blockGetBlock.Receipts, receipts)
	})
}
