package commands_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		var blockGetBlock types.Block
		require.NoError(t, json.Unmarshal([]byte(blockGetLine), &blockGetBlock))

		// ensure that we were returned the correct block

		require.Equal(t, minedBlockCidStr, blockGetBlock.Cid().String())

		// ensure that the JSON we received from block get conforms to schema

		requireSchemaConformance(t, []byte(blockGetLine), "filecoin_block")
	})
}

func TestShowDeal(t *testing.T) {
	tf.IntegrationTest(t)

	fastenvOpts := fast.EnvironmentOpts{}

	ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fastenvOpts)
	defer func() {
		require.NoError(t, env.Teardown(ctx))
	}()

	clientNode := env.GenesisMiner
	require.NoError(t, clientNode.MiningStart(ctx))

	minerNode := env.RequireNewNodeWithFunds(1000)

	// Connect the clientNode and the minerNode
	require.NoError(t, series.Connect(ctx, clientNode, minerNode))

	// Create a minerNode
	collateral := big.NewInt(500)           // FIL
	price := big.NewFloat(0.000000001)      // price per byte/block
	expiry := big.NewInt(24 * 60 * 60 / 30) // ~24 hours

	ask, err := series.CreateStorageMinerWithAsk(ctx, minerNode, collateral, price, expiry)
	require.NoError(t, err)

	// Create some data that is the full sector size and make it autoseal asap

	maxBytesi64 := int64(getMaxUserBytesPerStagedSector())
	dataReader := io.LimitReader(rand.Reader, maxBytesi64)
	_, deal, err := series.ImportAndStore(ctx, clientNode, ask, files.NewReaderFile(dataReader))
	require.NoError(t, err)

	t.Run("showDeal outputs correct information", func(t *testing.T) {
		showDeal, err := clientNode.ShowDeal(ctx, deal.ProposalCid)
		require.NoError(t, err)

		assert.Equal(t, ask.Miner, showDeal.Miner)
		assert.Equal(t, storagedeal.Accepted, showDeal.Response.State)

		duri64 := int64(showDeal.Proposal.Duration)
		foo := big.NewInt(duri64 * maxBytesi64)

		totalPrice := ask.Price.MulBigInt(foo)

		assert.Equal(t, totalPrice, showDeal.Proposal.TotalPrice)
	})

	t.Run("When deal does not exist says deal not found", func(t *testing.T) {
		deal.ProposalCid = requireTestCID(t, []byte("anything"))
		showDeal, err := clientNode.ShowDeal(ctx, deal.ProposalCid)
		assert.Error(t, err, "Error: deal not found")
		assert.Nil(t, showDeal)
	})

	t.Run("prints voucher info when deal has payment vouchers", func(t *testing.T) {

	})

	t.Run("prints 'none' if no vouchers", func(t *testing.T) {

	})
}

func getMaxUserBytesPerStagedSector() uint64 {
	return proofs.GetMaxUserBytesPerStagedSector(types.OneKiBSectorSize).Uint64()
}

func requireTestCID(t *testing.T, data []byte) cid.Cid {
	hash, err := multihash.Sum(data, multihash.SHA2_256, -1)
	require.NoError(t, err)
	return cid.NewCidV1(cid.DagCBOR, hash)
}
