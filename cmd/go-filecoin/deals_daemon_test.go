package commands_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"strings"
	"testing"
	"time"

	ffi "github.com/filecoin-project/filecoin-ffi"

	"github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commands "github.com/filecoin-project/go-filecoin/cmd/go-filecoin"
	"github.com/filecoin-project/go-filecoin/fixtures"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/protocol/storage/storagedeal"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/fastesting"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
)

func TestDealsRedeem(t *testing.T) {
	t.Skip("Long term solution: #3642")
	tf.IntegrationTest(t)

	ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fast.FilecoinOpts{
		DaemonOpts: []fast.ProcessDaemonOption{fast.POBlockTime(100 * time.Millisecond)},
	})

	defer func() {
		require.NoError(t, env.Teardown(ctx))
	}()

	clientDaemon := env.GenesisMiner
	minerDaemon := env.RequireNewNodeWithFunds(10000)

	collateral := big.NewInt(int64(1))
	price := big.NewFloat(float64(1))
	expiry := big.NewInt(int64(10000))

	pparams, err := minerDaemon.Protocol(ctx)
	require.NoError(t, err)

	sinfo := pparams.SupportedSectors[0]

	// mine the create storage message, then mine the set ask message
	series.CtxMiningNext(ctx, 2)

	_, err = series.CreateStorageMinerWithAsk(ctx, minerDaemon, collateral, price, expiry, sinfo.Size)
	require.NoError(t, err)

	require.NoError(t, minerDaemon.MiningSetup(ctx))

	f := files.NewBytesFile([]byte("HODLHODLHODL"))
	dataCid, err := clientDaemon.ClientImport(ctx, f)
	require.NoError(t, err)
	dataPriceOneBlock := uint64(12)

	var minerAddress address.Address
	err = minerDaemon.ConfigGet(ctx, "mining.minerAddress", &minerAddress)
	require.NoError(t, err)

	minerOwnerAddresses, err := minerDaemon.AddressLs(ctx)
	require.NoError(t, err)
	minerOwnerAddress := minerOwnerAddresses[0]

	dealDuration := uint64(5)

	// mine the createChannel message needed to create a storage proposal
	series.CtxMiningNext(ctx, 1)

	dealResponse, err := clientDaemon.ClientProposeStorageDeal(ctx, dataCid, minerAddress, 0, dealDuration)
	require.NoError(t, err)

	// atLeastStartH is either the start height of the deal or a height after the deal has started.
	atLeastStartH, err := series.GetHeadBlockHeight(ctx, clientDaemon)
	require.NoError(t, err)

	// Wait until deal is accepted so miner has redeemable vouchers.
	_, err = series.WaitForDealState(ctx, clientDaemon, dealResponse, storagedeal.Staged)
	require.NoError(t, err)

	// Wait until deal period is complete.
	completeHeight := types.NewBlockHeight(dealDuration).Add(atLeastStartH)
	for height := atLeastStartH; completeHeight.GreaterThan(height); {
		_, err := clientDaemon.MiningOnce(ctx)
		require.NoError(t, err)
		height, err = series.GetHeadBlockHeight(ctx, clientDaemon)
		require.NoError(t, err)
	}

	oldWalletBalance, err := minerDaemon.WalletBalance(ctx, minerOwnerAddress)
	require.NoError(t, err)

	// Note: we are racing against sealing here.  If sealing were to finish
	// after the wallet query but before we issue the redeem message then
	// our math will be off due to commitSector message gas and possible
	// block rewards.  In practice sealing takes much longer so we never
	// lose the race.
	redeemCid, err := minerDaemon.DealsRedeem(ctx, dealResponse.ProposalCid, fast.AOPrice(big.NewFloat(0.001)), fast.AOLimit(100))
	require.NoError(t, err)

	_, err = clientDaemon.MiningOnce(ctx)
	require.NoError(t, err)

	_, err = minerDaemon.MessageWait(ctx, redeemCid)
	require.NoError(t, err)

	newWalletBalance, err := minerDaemon.WalletBalance(ctx, minerOwnerAddress)
	require.NoError(t, err)

	expectedGasCost := 0.1
	expectedBalanceDiff := float64(dealDuration*dataPriceOneBlock) - expectedGasCost
	expectedBalanceStr := fmt.Sprintf("%.1f", expectedBalanceDiff)

	actualBalanceDiff := newWalletBalance.Sub(oldWalletBalance)
	assert.Equal(t, expectedBalanceStr, actualBalanceDiff.String())
}

func TestDealsList(t *testing.T) {
	t.Skip("Long term solution: #3642")
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
	dealCid1 := splitOnSpace[len(splitOnSpace)-1]

	// create another deal with zero price
	dataCid = clientDaemon.RunWithStdin(strings.NewReader("FREEASINBEER"), "client", "import").ReadStdoutTrimNewlines()
	addAskCid = minerDaemon.MinerSetPrice(fixtures.TestMiners[0], fixtures.TestAddresses[0], "0", "10")
	clientDaemon.WaitForMessageRequireSuccess(addAskCid)
	proposeDealOutput = clientDaemon.RunSuccess("client", "propose-storage-deal", fixtures.TestMiners[0], dataCid, "1", "5").ReadStdoutTrimNewlines()
	splitOnSpace = strings.Split(proposeDealOutput, " ")
	dealCid2 := splitOnSpace[len(splitOnSpace)-1]

	t.Run("with no filters", func(t *testing.T) {
		// Client sees both deals
		clientOutput := clientDaemon.RunSuccess("deals", "list").ReadStdoutTrimNewlines()
		assert.Contains(t, clientOutput, dealCid1)
		assert.Contains(t, clientOutput, dealCid2)

		// Miner sees the deal
		minerOutput := minerDaemon.RunSuccess("deals", "list").ReadStdoutTrimNewlines()
		assert.Contains(t, minerOutput, dealCid1)
		assert.Contains(t, minerOutput, dealCid2)
	})

	t.Run("with --miner", func(t *testing.T) {
		// Client does not see the deal
		clientOutput := clientDaemon.RunSuccess("deals", "list", "--miner").ReadStdoutTrimNewlines()
		assert.NotContains(t, clientOutput, dealCid1)

		// Miner sees the deal
		minerOutput := minerDaemon.RunSuccess("deals", "list", "--miner").ReadStdoutTrimNewlines()
		assert.Contains(t, minerOutput, dealCid1)
	})

	t.Run("with --client", func(t *testing.T) {
		// Client sees the deal
		clientOutput := clientDaemon.RunSuccess("deals", "list", "--client").ReadStdoutTrimNewlines()
		assert.Contains(t, clientOutput, dealCid1)

		// Miner does not see the deal
		minerOutput := minerDaemon.RunSuccess("deals", "list", "--client").ReadStdoutTrimNewlines()
		assert.NotContains(t, minerOutput, dealCid1)
	})

	t.Run("with --help", func(t *testing.T) {
		clientOutput := clientDaemon.RunSuccess("deals", "list", "--help").ReadStdoutTrimNewlines()
		assert.Contains(t, clientOutput, "only return deals made as a client")
		assert.Contains(t, clientOutput, "only return deals made as a miner")
	})
}

func TestDealsShow(t *testing.T) {
	t.Skip("Long term solution: #3642")
	tf.IntegrationTest(t)

	// increase block time to give it it a chance to seal
	opts := fast.FilecoinOpts{
		DaemonOpts: []fast.ProcessDaemonOption{fast.POBlockTime(100 * time.Millisecond)},
	}

	ctx, env := fastesting.NewTestEnvironment(context.Background(), t, opts)
	defer func() {
		require.NoError(t, env.Teardown(ctx))
	}()

	clientNode := env.GenesisMiner
	minerNode := env.RequireNewNodeWithFunds(1000)

	// Connect the clientNode and the minerNode
	require.NoError(t, series.Connect(ctx, clientNode, minerNode))

	// Create a minerNode
	collateral := big.NewInt(500)           // FIL
	price := big.NewFloat(0.000000001)      // price per byte/block
	expiry := big.NewInt(24 * 60 * 60 / 30) // ~24 hours

	pparams, err := minerNode.Protocol(ctx)
	require.NoError(t, err)

	sinfo := pparams.SupportedSectors[0]

	// mine the create storage message, then mine the set ask message
	series.CtxMiningNext(ctx, 2)

	ask, err := series.CreateStorageMinerWithAsk(ctx, minerNode, collateral, price, expiry, sinfo.Size)
	require.NoError(t, err)

	// enable storage protocol
	err = minerNode.MiningSetup(ctx)
	require.NoError(t, err)

	// Create some data that is the full sector size and make it autoseal asap

	maxBytesi64 := int64(getMaxUserBytesPerStagedSector())
	dataReader := io.LimitReader(rand.Reader, maxBytesi64)

	// mine the createChannel message needed to create a storage proposal
	series.CtxMiningNext(ctx, 1)

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
	t.Skip("Long term solution: #3642")
	tf.IntegrationTest(t)

	// increase block time to give it it a chance to seal
	opts := fast.FilecoinOpts{
		DaemonOpts: []fast.ProcessDaemonOption{fast.POBlockTime(100 * time.Millisecond)},
	}
	ctx, env := fastesting.NewTestEnvironment(context.Background(), t, opts)

	// Teardown after test ends
	defer func() {
		require.NoError(t, env.Teardown(ctx))
	}()

	maxBytesi64 := int64(getMaxUserBytesPerStagedSector())

	// Use a longer duration so we can have >1 voucher to test
	durationui64 := uint64(2000)
	price := big.NewFloat(0.000000001) // price per byte/block
	clientNode, ask, clientAddr, deal := setupDeal(ctx, t, env, price, durationui64, maxBytesi64)

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
				Index:  0,
				Amount: &firstAmount,
				Condition: &types.Predicate{
					Method: miner.VerifyPieceInclusion,
					To:     ask.Miner,
				},
				Payer:   &clientAddr,
				ValidAt: types.NewBlockHeight(durationui64 / 2),
			},
			{
				Index:     1,
				Amount:    totalPrice,
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

func TestFreeDealsShowPaymentVouchers(t *testing.T) {
	t.Skip("Long term solution: #3642")
	tf.IntegrationTest(t)

	// increase block time to give it it a chance to seal
	opts := fast.FilecoinOpts{
		DaemonOpts: []fast.ProcessDaemonOption{fast.POBlockTime(100 * time.Millisecond)},
	}
	ctx, env := fastesting.NewTestEnvironment(context.Background(), t, opts)

	// Teardown after test ends
	defer func() {
		require.NoError(t, env.Teardown(ctx))
	}()

	maxBytesi64 := int64(getMaxUserBytesPerStagedSector())

	// Use a longer duration so we can have >1 voucher to test
	durationui64 := uint64(2000)
	price := big.NewFloat(0) // free deal
	clientNode, _, _, deal := setupDeal(ctx, t, env, price, durationui64, maxBytesi64)

	t.Run("No vouchers doesn't break output", func(t *testing.T) {
		res, err := clientNode.DealsShow(ctx, deal.ProposalCid)
		require.NoError(t, err)

		expected := []*commands.PaymenVoucherResult{}

		assertEqualVoucherResults(t, expected, res.PaymentVouchers)
	})
}

func setupDeal(
	ctx context.Context,
	t *testing.T,
	env *fastesting.TestEnvironment,
	price *big.Float,
	duration uint64,
	maxBytes int64,
) (*fast.Filecoin, porcelain.Ask, address.Address, *storagedeal.Response) {

	clientNode := env.GenesisMiner

	minerNode := env.RequireNewNodeWithFunds(1000)

	// Connect the clientNode and the minerNode
	require.NoError(t, series.Connect(ctx, clientNode, minerNode))

	// Create a minerNode
	collateral := big.NewInt(500)           // FIL
	expiry := big.NewInt(24 * 60 * 60 / 30) // ~24 hours

	pparams, err := minerNode.Protocol(ctx)
	require.NoError(t, err)

	sinfo := pparams.SupportedSectors[0]

	// mine the create storage message, then mine the set ask message
	series.CtxMiningNext(ctx, 2)

	// This also starts the Miner.
	ask, err := series.CreateStorageMinerWithAsk(ctx, minerNode, collateral, price, expiry, sinfo.Size)
	require.NoError(t, err)
	require.NoError(t, minerNode.MiningStop(ctx))

	// Setup miner to listen to storage deals
	err = minerNode.MiningSetup(ctx)
	require.NoError(t, err)

	// Create some data that is the full sector size and make it autoseal asap
	dataReader := io.LimitReader(rand.Reader, maxBytes)

	var clientAddr address.Address
	err = clientNode.ConfigGet(ctx, "wallet.defaultAddress", &clientAddr)
	require.NoError(t, err)

	// Mine the createChannel message needed to create a storage proposal.
	// The channel will only be created if the price is greater than zero.
	if price.Cmp(big.NewFloat(0)) > 0 {
		series.CtxMiningNext(ctx, 1)
	}

	_, deal, err := series.ImportAndStoreWithDuration(ctx, clientNode, ask, duration, files.NewReaderFile(dataReader))
	require.NoError(t, err)
	require.NoError(t, clientNode.MiningStop(ctx))

	return clientNode, ask, clientAddr, deal
}

func getMaxUserBytesPerStagedSector() uint64 {
	return ffi.GetMaxUserBytesPerStagedSector(types.OneKiBSectorSize.Uint64())
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
	var channelID *types.ChannelID
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
		assert.True(t, vr.ValidAt.LessEqual(actual[i].ValidAt), "expva %s, actualva %s", vr.ValidAt.String(), actual[i].Channel.String())

		// verify channel ids exist and are the same
		if channelID == nil {
			assert.NotNil(t, actual[i].Channel)
			channelID = vr.Channel
		} else {
			assert.True(t, channelID.Equal(actual[i].Channel), "expch %s, actualch %s", channelID.String(), actual[i].Channel.String())
		}
	}
}
