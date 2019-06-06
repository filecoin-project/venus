package commands_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/filecoin-project/go-filecoin/fixtures"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/fastesting"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"math/big"
	"strings"
	"testing"
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

	minerDaemon := th.NewDaemon(t,
		th.WithMiner(fixtures.TestMiners[0]),
		th.KeyFile(fixtures.KeyFilePaths()[0]),
		th.DefaultAddress(fixtures.TestAddresses[0]),
		th.AutoSealInterval("1"),
	).Start()
	defer minerDaemon.ShutdownSuccess()

	clientDaemon := th.NewDaemon(t,
		th.KeyFile(fixtures.KeyFilePaths()[1]),
		th.DefaultAddress(fixtures.TestAddresses[1]),
	).Start()
	defer clientDaemon.ShutdownSuccess()

	minerDaemon.RunSuccess("mining", "start")
	minerDaemon.UpdatePeerID()

	minerDaemon.ConnectSuccess(clientDaemon)

	addAskCid := minerDaemon.MinerSetPrice(fixtures.TestMiners[0], fixtures.TestAddresses[0], "20", "10")

	t.Run("when deal exists shows deal info", func(t *testing.T) {
		clientDaemon.WaitForMessageRequireSuccess(addAskCid)
		dataCid := clientDaemon.RunWithStdin(strings.NewReader("HODLHODLHODL"), "client", "import").ReadStdoutTrimNewlines()

		proposeDealOutput := clientDaemon.RunSuccess("client", "propose-storage-deal", fixtures.TestMiners[0], dataCid, "0", "5").ReadStdoutTrimNewlines()

		splitOnSpace := strings.Split(proposeDealOutput, " ")

		dealCid := splitOnSpace[len(splitOnSpace)-1]

		showdeal := minerDaemon.RunSuccess("show", "deal", dealCid).ReadStdoutTrimNewlines()
		assert.Contains(t, showdeal, fmt.Sprintf("CID: %s", dealCid))
		assert.Contains(t, showdeal, fmt.Sprintf("Miner: %s", fixtures.TestMiners[0]))
		assert.Contains(t, showdeal, "Duration: 5 blocks")
		assert.Contains(t, showdeal, "State: staged")
		assert.Contains(t, showdeal, "Size: 12 bytes")
		assert.Contains(t, showdeal, "Total Price: 1200 FIL")
	})

	t.Run("When deal does not exist says deal not found", func(t *testing.T) {
		expected := "deal not found"
		minerDaemon.RunFail(expected, "show", "deal", addAskCid.String()).ReadStdoutTrimNewlines()
	})
}

func TestShowDeal2(t *testing.T) {
	tf.IntegrationTest(t)

	fastenvOpts := fast.EnvironmentOpts{
		//InitOpts: []fast.ProcessInitOption{fast.POAutoSealIntervalSeconds(1)},
		//DaemonOpts: []fast.ProcessDaemonOption{fast.POBlockTime(time.Second)},
	}

	ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fastenvOpts)

	clientNode := env.GenesisMiner
	require.NoError(t, clientNode.MiningStart(ctx))

	minerNode := env.RequireNewNodeWithFunds(1000)
	defer func() {
		require.NoError(t, env.Teardown(ctx))
	}()

	// Connect the clientNode and the minerNode
	err := series.Connect(ctx, clientNode, minerNode)
	require.NoError(t, err)

	// Create a minerNode
	collateral := big.NewInt(500)           // FIL
	price := big.NewFloat(0.000000001)      // price per byte/block
	expiry := big.NewInt(24 * 60 * 60 / 30) // ~24 hours

	ask, err := series.CreateStorageMinerWithAsk(ctx, minerNode, collateral, price, expiry)
	require.NoError(t, err)

	// Create some data that is the full sector size and make it autoseal asap
	var data bytes.Buffer
	dataReader := io.LimitReader(rand.Reader, int64(getMaxUserBytesPerStagedSector()))
	dataReader = io.TeeReader(dataReader, &data)
	_, deal, err := series.ImportAndStore(ctx, clientNode, ask, files.NewReaderFile(dataReader))
	require.NoError(t, err)

	t.Run("showDeal outputs correct information", func(t *testing.T) {
		showDeal, err := clientNode.ShowDeal(ctx, deal)
		require.NoError(t, err)

		assert.Equal(t, ask.Miner.String(), showDeal.Miner.String())
		assert.Equal(t, "accepted", showDeal.Response.State.String())

		totalPrice := types.NewAttoFILFromFIL(1016)
		assert.Equal(t, totalPrice, showDeal.Proposal.TotalPrice)
	})

	t.Run("When deal does not exist says deal not found", func(t *testing.T) {
		deal.ProposalCid = requireTestCID(t, []byte("anything"))
		showDeal, err := clientNode.ShowDeal(ctx, deal)
		assert.Error(t, err, "Error: deal not found")
		assert.Nil(t, showDeal)
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
