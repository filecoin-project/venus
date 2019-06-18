package commands_test

import (
	"context"
	"crypto/rand"
	"io"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs-files"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/fixtures"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/fastesting"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestDealsRedeem(t *testing.T) {
	tf.IntegrationTest(t)

	ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fast.FilecoinOpts{})

	defer func() {
		require.NoError(t, env.Teardown(ctx))
	}()

	clientDaemon := env.GenesisMiner
	minerDaemon := env.RequireNewNodeWithFunds(10000)

	require.NoError(t, clientDaemon.MiningStart(ctx))

	collateral := big.NewInt(int64(1))
	price := big.NewFloat(float64(1))
	expiry := big.NewInt(int64(10000))
	_, err := series.CreateStorageMinerWithAsk(ctx, minerDaemon, collateral, price, expiry)
	require.NoError(t, err)

	f := files.NewBytesFile([]byte("HODLHODLHODL"))
	dataCid, err := clientDaemon.ClientImport(ctx, f)
	require.NoError(t, err)

	var minerAddress address.Address
	err = minerDaemon.ConfigGet(ctx, "mining.minerAddress", &minerAddress)
	require.NoError(t, err)

	dealResponse, err := clientDaemon.ClientProposeStorageDeal(ctx, dataCid, minerAddress, 0, 1, true)
	require.NoError(t, err)

	err = series.WaitForDealState(ctx, clientDaemon, dealResponse, storagedeal.Posted)
	require.NoError(t, err)

	// Stop mining to guarantee the miner doesn't receive any block rewards
	require.NoError(t, minerDaemon.MiningStop(ctx))
	// Wait for 1 blocktime to allow any remaining block rewards to be processed
	protocolDetails, err := minerDaemon.Protocol(ctx)
	require.NoError(t, err)
	time.Sleep(protocolDetails.BlockTime)

	minerOwnerAddresses, err := minerDaemon.AddressLs(ctx)
	require.NoError(t, err)
	minerOwnerAddress := minerOwnerAddresses[0]

	oldWalletBalance, err := minerDaemon.WalletBalance(ctx, minerOwnerAddress)
	require.NoError(t, err)

	redeemCid, err := minerDaemon.DealsRedeem(ctx, dealResponse.ProposalCid, fast.AOPrice(big.NewFloat(0.001)), fast.AOLimit(100))
	require.NoError(t, err)

	_, err = minerDaemon.MessageWait(ctx, redeemCid)
	require.NoError(t, err)

	newWalletBalance, err := minerDaemon.WalletBalance(ctx, minerOwnerAddress)
	require.NoError(t, err)

	actualBalanceDiff := newWalletBalance.Sub(oldWalletBalance)
	assert.Equal(t, "11.9", actualBalanceDiff.String())
}

func TestDealsList(t *testing.T) {
	tf.IntegrationTest(t)

	clientDaemon := th.NewDaemon(t,
		th.KeyFile(fixtures.KeyFilePaths()[1]),
		th.DefaultAddress(fixtures.TestAddresses[1]),
	).Start()
	defer clientDaemon.ShutdownSuccess()

	minerDaemon := th.NewDaemon(t,
		th.WithMiner(fixtures.TestMiners[0]),
		th.KeyFile(fixtures.KeyFilePaths()[0]),
		th.DefaultAddress(fixtures.TestAddresses[0]),
		th.AutoSealInterval("1"),
	).Start()
	defer minerDaemon.ShutdownSuccess()

	minerDaemon.RunSuccess("mining", "start")
	minerDaemon.UpdatePeerID()

	minerDaemon.ConnectSuccess(clientDaemon)

	// Create a deal from the client daemon to the miner daemon
	addAskCid := minerDaemon.MinerSetPrice(fixtures.TestMiners[0], fixtures.TestAddresses[0], "20", "10")
	clientDaemon.WaitForMessageRequireSuccess(addAskCid)
	dataCid := clientDaemon.RunWithStdin(strings.NewReader("HODLHODLHODL"), "client", "import").ReadStdoutTrimNewlines()
	proposeDealOutput := clientDaemon.RunSuccess("client", "propose-storage-deal", fixtures.TestMiners[0], dataCid, "0", "5").ReadStdoutTrimNewlines()
	splitOnSpace := strings.Split(proposeDealOutput, " ")
	dealCid := splitOnSpace[len(splitOnSpace)-1]

	t.Run("with no filters", func(t *testing.T) {
		// Client sees the deal
		clientOutput := clientDaemon.RunSuccess("deals", "list").ReadStdoutTrimNewlines()
		assert.Contains(t, clientOutput, dealCid)

		// Miner sees the deal
		minerOutput := minerDaemon.RunSuccess("deals", "list").ReadStdoutTrimNewlines()
		assert.Contains(t, minerOutput, dealCid)
	})

	t.Run("with --miner", func(t *testing.T) {
		// Client does not see the deal
		clientOutput := clientDaemon.RunSuccess("deals", "list", "--miner").ReadStdoutTrimNewlines()
		assert.NotContains(t, clientOutput, dealCid)

		// Miner sees the deal
		minerOutput := minerDaemon.RunSuccess("deals", "list", "--miner").ReadStdoutTrimNewlines()
		assert.Contains(t, minerOutput, dealCid)
	})

	t.Run("with --client", func(t *testing.T) {
		// Client sees the deal
		clientOutput := clientDaemon.RunSuccess("deals", "list", "--client").ReadStdoutTrimNewlines()
		assert.Contains(t, clientOutput, dealCid)

		// Miner does not see the deal
		minerOutput := minerDaemon.RunSuccess("deals", "list", "--client").ReadStdoutTrimNewlines()
		assert.NotContains(t, minerOutput, dealCid)
	})

	t.Run("with --help", func(t *testing.T) {
		clientOutput := clientDaemon.RunSuccess("deals", "list", "--help").ReadStdoutTrimNewlines()
		assert.Contains(t, clientOutput, "only return deals made as a client")
		assert.Contains(t, clientOutput, "only return deals made as a miner")
	})
}

func TestShowDeal(t *testing.T) {
	tf.IntegrationTest(t)

	fastenvOpts := fast.FilecoinOpts{}

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
		showDeal, err := clientNode.DealsShow(ctx, deal.ProposalCid)
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
		showDeal, err := clientNode.DealsShow(ctx, deal.ProposalCid)
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
