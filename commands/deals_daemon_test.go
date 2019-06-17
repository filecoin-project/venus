package commands_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs-files"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/commands"
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
	// DISABLED: this test has nondeterministic rounding errors
	// https://github.com/filecoin-project/go-filecoin/issues/2960
	// It also takes many minutes due to waiting for real sector sealing. This is unacceptable
	// for an integration test (possibly ok for a functional test).
	// https://github.com/filecoin-project/go-filecoin/issues/2965
	t.Skipf("Flaky and slow: #2960, #2965")
	tf.IntegrationTest(t)

	ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fast.FilecoinOpts{})

	defer func() {
		require.NoError(t, env.Teardown(ctx))
	}()

	clientDaemon := env.GenesisMiner
	minerDaemon := env.RequireNewNodeWithFunds(10000)

	require.NoError(t, clientDaemon.MiningStart(ctx))
	defer func() {
		require.NoError(t, clientDaemon.MiningStop(ctx))
	}()

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

	// this is to fix flaky test failures due to the amount being 11.8999999
	actualBalanceDiff := newWalletBalance.Sub(oldWalletBalance)
	rounded, err := strconv.ParseFloat(actualBalanceDiff.String(), 32)
	require.NoError(t, err)
	assert.Equal(t, "11.9", fmt.Sprintf("%3.1f", rounded))
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

func TestDealsShow(t *testing.T) {
	tf.IntegrationTest(t)

	ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fast.FilecoinOpts{})
	defer func() {
		require.NoError(t, env.Teardown(ctx))
	}()

	clientNode := env.GenesisMiner
	require.NoError(t, clientNode.MiningStart(ctx))
	defer func() {
		require.NoError(t, clientNode.MiningStop(ctx))
	}()

	minerNode := env.RequireNewNodeWithFunds(1000)

	// Connect the clientNode and the minerNode
	require.NoError(t, series.Connect(ctx, clientNode, minerNode))

	// Create a minerNode
	collateral := big.NewInt(500)           // FIL
	price := big.NewFloat(0.000000001)      // price per byte/block
	expiry := big.NewInt(24 * 60 * 60 / 30) // ~24 hours

	ask, err := series.CreateStorageMinerWithAsk(ctx, minerNode, collateral, price, expiry)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, minerNode.MiningStop(ctx))
	}()

	// Create some data that is the full sector size and make it autoseal asap

	maxBytesi64 := int64(getMaxUserBytesPerStagedSector())
	dataReader := io.LimitReader(rand.Reader, maxBytesi64)

	// Duration is 10
	_, deal, err := series.ImportAndStore(ctx, clientNode, ask, files.NewReaderFile(dataReader))
	require.NoError(t, err)

	t.Run("showDeal outputs correct information", func(t *testing.T) {
		res, err := clientNode.DealsShow(ctx, deal.ProposalCid)
		require.NoError(t, err)

		assert.Equal(t, uint64(10), res.Duration)
		assert.Equal(t, ask.Miner.String(), res.Miner.String())
		assert.Equal(t, storagedeal.Accepted, res.State)

		duri64 := int64(res.Duration)
		durXmax := big.NewInt(duri64 * maxBytesi64)

		totalPrice := ask.Price.MulBigInt(durXmax)

		assert.True(t, totalPrice.Equal(*res.TotalPrice))
	})

	t.Run("When deal does not exist says deal not found", func(t *testing.T) {
		deal.ProposalCid = requireTestCID(t, []byte("anything"))
		showDeal, err := clientNode.DealsShow(ctx, deal.ProposalCid)
		assert.Error(t, err, "Error: deal not found")
		assert.Nil(t, showDeal)
	})

}

func TestDealsShowPaymentVouchers(t *testing.T) {
	tf.IntegrationTest(t)

	ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fast.FilecoinOpts{})
	// Teardown after test ends
	defer func() {
		require.NoError(t, env.Teardown(ctx))
	}()

	clientNode := env.GenesisMiner
	require.NoError(t, clientNode.MiningStart(ctx))
	defer func() {
		assert.NoError(t, clientNode.MiningStop(ctx))
	}()

	minerNode := env.RequireNewNodeWithFunds(1000)

	// Connect the clientNode and the minerNode
	require.NoError(t, series.Connect(ctx, clientNode, minerNode))

	// Create a minerNode
	collateral := big.NewInt(500)           // FIL
	price := big.NewFloat(0.000000001)      // price per byte/block
	expiry := big.NewInt(24 * 60 * 60 / 30) // ~24 hours

	// This also starts the Miner
	ask, err := series.CreateStorageMinerWithAsk(ctx, minerNode, collateral, price, expiry)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, minerNode.MiningStop(ctx))
	}()

	// Create some data that is the full sector size and make it autoseal asap
	maxBytesi64 := int64(getMaxUserBytesPerStagedSector())
	dataReader := io.LimitReader(rand.Reader, maxBytesi64)

	var clientAddr address.Address
	err = clientNode.ConfigGet(ctx, "wallet.defaultAddress", &clientAddr)
	require.NoError(t, err)

	// Use a longer duration so we can have >1 voucher to test
	durationui64 := uint64(2000)

	_, deal, err := series.ImportAndStoreWithDuration(ctx, clientNode, ask, durationui64, files.NewReaderFile(dataReader))
	require.NoError(t, err)

	require.NoError(t, minerNode.MiningStop(ctx))
	require.NoError(t, clientNode.MiningStop(ctx))

	t.Run("Vouchers output as JSON have the correct info", func(t *testing.T) {
		res, err := clientNode.DealsShow(ctx, deal.ProposalCid)
		require.NoError(t, err)

		totalPrice := calcTotalPrice(big.NewInt(int64(durationui64)), maxBytesi64, &ask.Price)

		provingPeriods, _ := types.NewAttoFILFromString("2", 10)
		firstAmount := totalPrice.DivCeil(provingPeriods)

		// ValidAt block height should be at least as high as the (period index + 1) * duration / # of proving periods
		// so if there are 2 periods, 1 is valid at block height >= 1*duration/2,
		// 2 is valid at 2*duration/2
		expected := []*commands.PaymenVoucherResult{
			{
				Index:   0,
				Amount:  &firstAmount,
				Channel: types.NewChannelID(4),
				Condition: &types.Predicate{
					Method: "verifyPieceInclusion",
					To:     ask.Miner,
				},
				Payer:   &clientAddr,
				ValidAt: types.NewBlockHeight(durationui64 / 2),
			},
			{
				Index:     1,
				Amount:    totalPrice,
				Channel:   types.NewChannelID(4),
				Condition: nil,
				Payer:     &clientAddr,
				ValidAt:   types.NewBlockHeight(durationui64),
			},
		}

		assertEqualVoucherResults(t, expected, res.PaymentVouchers)

		assert.NotNil(t, res.PaymentVouchers[0].EncodedAs)
		assert.NotNil(t, res.PaymentVouchers[1].EncodedAs)
		assert.NotEqual(t, res.PaymentVouchers[0].EncodedAs, res.PaymentVouchers[1].EncodedAs)
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

func calcTotalPrice(duration *big.Int, maxBytes int64, price *types.AttoFIL) *types.AttoFIL {
	bXB := duration.Mul(duration, big.NewInt(maxBytes))
	res := price.MulBigInt(bXB)
	return &res
}

func assertEqualVoucherResults(t *testing.T, expected, actual []*commands.PaymenVoucherResult) {
	require.Len(t, actual, len(expected))
	for i, vr := range expected {
		assert.Equal(t, vr.Index, actual[i].Index)
		assert.Equal(t, vr.Payer.String(), actual[i].Payer.String())

		if vr.Condition != nil {
			assert.Equal(t, vr.Condition.Method, actual[i].Condition.Method)
			assert.Equal(t, vr.Condition.To.String(), actual[i].Condition.To.String())
		} else {
			assert.Nil(t, actual[i].Condition)
		}

		assert.True(t, vr.Amount.Equal(*actual[i].Amount))
		assert.True(t, vr.ValidAt.LessEqual(actual[i].ValidAt))
		assert.True(t, vr.Channel.Equal(actual[i].Channel))
	}
}
