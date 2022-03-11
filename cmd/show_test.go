package cmd_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/filecoin-project/venus/pkg/testhelpers"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/app/node/test"
	"github.com/filecoin-project/venus/fixtures/fortest"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

// Receipt is what is returned by executing a message on the vm.
type Receipt struct {
	// control field for encoding struct as an array
	_        struct{} `cbor:",toarray"`
	ExitCode int64    `json:"exitCode"`
	Return   []byte   `json:"return"`
	GasUsed  int64    `json:"gasUsed"`
}

func TestBlockDaemon(t *testing.T) {
	tf.IntegrationTest(t)
	t.Skip("Unskip with fake proofs")

	t.Run("show block <cid-of-genesis-block> returns human readable output for the filecoin block", func(t *testing.T) {
		ctx := context.Background()
		builder := test.NewNodeBuilder(t)
		buildWithMiner(t, builder)

		_, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		// mine a block and get its CID
		mockBlk, err := mockBlock(t)
		require.NoError(t, err)
		//minedBlockCid := minedBlock.Cid()
		blkCid := mockBlk.Cid()
		// get the mined block by its CID
		output := cmdClient.RunSuccess(ctx, "show", "block", blkCid.String()).ReadStdoutTrimNewlines()

		assert.Contains(t, output, "BlockHeader Details")
		assert.Contains(t, output, "Weight: 0")
		assert.Contains(t, output, "Height: 1")
		assert.Contains(t, output, "Timestamp:  ")
	})

	t.Run("show block --messages <cid-of-genesis-block> returns human readable output for the filecoin block including messages", func(t *testing.T) {
		ctx := context.Background()
		builder := test.NewNodeBuilder(t)
		buildWithMiner(t, builder)

		_, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		// mine a block and get its CID
		//minedBlock, err := n.BlockMining.BlockMiningAPI.MiningOnce(ctx)
		mockBlk, err := mockBlock(t)
		require.NoError(t, err)

		// get the mined block by its CID
		output := cmdClient.RunSuccess(ctx, "show", "block", "--messages", mockBlk.Cid().String()).ReadStdoutTrimNewlines()

		assert.Contains(t, output, "BlockHeader Details")
		assert.Contains(t, output, "Weight: 0")
		assert.Contains(t, output, "Height: 1")
		assert.Contains(t, output, "Timestamp:  ")
		assert.Contains(t, output, "Messages:  ")
	})

	t.Run("show block <cid-of-genesis-block> --enc json returns JSON for a filecoin block", func(t *testing.T) {
		ctx := context.Background()
		builder := test.NewNodeBuilder(t)
		buildWithMiner(t, builder)

		_, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		// mine a block and get its CID
		//minedBlock, err := n.BlockMining.BlockMiningAPI.MiningOnce(ctx)
		mockBlk, err := mockBlock(t)
		require.NoError(t, err)

		// get the mined block by its CID
		blockGetLine := cmdClient.RunSuccessFirstLine(ctx, "show", "block", mockBlk.Cid().String(), "--enc", "json")
		var blockGetBlock types.FullBlock
		require.NoError(t, json.Unmarshal([]byte(blockGetLine), &blockGetBlock))

		// ensure that we were returned the correct block

		require.Equal(t, mockBlk.Cid().String(), blockGetBlock.Header.Cid().String())
	})

	t.Run("show header <cid-of-genesis-block> --enc json returns JSON for a filecoin block header", func(t *testing.T) {
		ctx := context.Background()
		builder := test.NewNodeBuilder(t)
		buildWithMiner(t, builder)

		_, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		// mine a block and get its CID
		//minedBlock, err := n.BlockMining.BlockMiningAPI.MiningOnce(ctx)
		mockBlk, err := mockBlock(t)
		require.NoError(t, err)

		// get the mined block by its CID
		headerGetLine := cmdClient.RunSuccessFirstLine(ctx, "show", "header", mockBlk.Cid().String(), "--enc", "json")

		var headerGetBlock types.BlockHeader
		require.NoError(t, json.Unmarshal([]byte(headerGetLine), &headerGetBlock))

		// ensure that we were returned the correct block

		require.Equal(t, mockBlk.Cid().String(), headerGetBlock.Cid().String())
	})

	t.Run("show messages <empty-collection-cid> returns empty message collection", func(t *testing.T) {
		ctx := context.Background()
		builder := test.NewNodeBuilder(t)
		buildWithMiner(t, builder)

		_, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		//_, err := n.BlockMining.BlockMiningAPI.MiningOnce(ctx)
		_, err := mockBlock(t)
		require.NoError(t, err)

		emptyMessagesLine := cmdClient.RunSuccessFirstLine(ctx, "show", "messages", testhelpers.EmptyMessagesCID.String(), "--enc", "json")

		var messageCollection []*types.SignedMessage
		require.NoError(t, json.Unmarshal([]byte(emptyMessagesLine), &messageCollection))

		assert.Equal(t, 0, len(messageCollection))
	})

	t.Run("show receipts <empty-collection-cid> returns empty receipt collection", func(t *testing.T) {
		ctx := context.Background()
		builder := test.NewNodeBuilder(t)
		buildWithMiner(t, builder)

		_, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		//_, err := n.BlockMining.BlockMiningAPI.MiningOnce(ctx)
		_, err := mockBlock(t)
		require.NoError(t, err)

		emptyReceiptsLine := cmdClient.RunSuccessFirstLine(ctx, "show", "receipts", testhelpers.EmptyReceiptsCID.String(), "--enc", "json")

		var receipts []Receipt
		require.NoError(t, json.Unmarshal([]byte(emptyReceiptsLine), &receipts))

		assert.Equal(t, 0, len(receipts))
	})

	t.Run("show messages", func(t *testing.T) {
		cs := test.FixtureChainSeed(t)
		defaultAddr := fortest.TestAddresses[0]
		ctx := context.Background()
		builder := test.NewNodeBuilder(t)
		builder.WithGenesisInit(cs.GenesisInitFunc)
		//builder.WithConfig(cs.MinerConfigOpt(0))
		builder.WithConfig(test.DefaultAddressConfigOpt(defaultAddr))
		builder.WithInitOpt(cs.KeyInitOpt(1))
		builder.WithInitOpt(cs.KeyInitOpt(0))

		n, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		from, err := n.Wallet().API().WalletDefaultAddress(ctx) // this should = fixtures.TestAddresses[0]
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

		//blk, err := n.BlockMining.BlockMiningAPI.MiningOnce(ctx)
		mockBlk, err := mockBlock(t)
		require.NoError(t, err)

		// Full block checks out
		blockGetLine := cmdClient.RunSuccessFirstLine(ctx, "show", "block", mockBlk.Cid().String(), "--enc", "json")
		var blockGetBlock types.FullBlock
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

func mockBlock(t *testing.T) (*types.BlockHeader, error) {
	b := &types.BlockHeader{
		Miner:         testhelpers.NewForTestGetter()(),
		Ticket:        &types.Ticket{VRFProof: []byte{0x01, 0x02, 0x03}},
		ElectionProof: &types.ElectionProof{VRFProof: []byte{0x0a, 0x0b}},
		Height:        2,
		BeaconEntries: []types.BeaconEntry{
			{
				Round: 1,
				Data:  []byte{0x3},
			},
		},
		Messages:              testhelpers.CidFromString(t, "somecid"),
		ParentMessageReceipts: testhelpers.CidFromString(t, "somecid"),
		Parents:               []cid.Cid{testhelpers.CidFromString(t, "somecid")},
		ParentWeight:          big.NewInt(1000),
		ParentStateRoot:       testhelpers.CidFromString(t, "somecid"),
		Timestamp:             1,
		BlockSig: &crypto.Signature{
			Type: crypto.SigTypeBLS,
			Data: []byte{0x3},
		},
		BLSAggregate: &crypto.Signature{
			Type: crypto.SigTypeBLS,
			Data: []byte{0x3},
		},
		// PoStProofs:    posts,
		ForkSignaling: 6,
	}
	return b, nil
}
