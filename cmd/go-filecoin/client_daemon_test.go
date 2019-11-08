package commands_test

import (
	"bytes"
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ipfs/go-ipfs-files"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/fixtures"
	"github.com/filecoin-project/go-filecoin/internal/pkg/protocol/storage/storagedeal"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/fastesting"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
)

func TestListAsks(t *testing.T) {
	t.Skip("Long term solution: #3642")
	tf.IntegrationTest(t)

	minerDaemon := makeTestDaemonWithMinerAndStart(t)
	defer minerDaemon.ShutdownSuccess()

	minerDaemon.RunSuccess("mining start")
	minerDaemon.MinerSetPrice(fixtures.TestMiners[0], fixtures.TestAddresses[0], "20", "10")

	listAsksOutput := minerDaemon.RunSuccess("client", "list-asks").ReadStdoutTrimNewlines()
	assert.Equal(t, fixtures.TestMiners[0]+" 000 20 11", listAsksOutput)
}

func TestStorageDealsAfterRestart(t *testing.T) {
	t.Skip("Long term solution: #3642")
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
	clientDaemon.WaitForMessageRequireSuccess(addAskCid)
	dataCid := clientDaemon.RunWithStdin(strings.NewReader("HODLHODLHODL"), "client", "import").ReadStdoutTrimNewlines()

	proposeDealOutput := clientDaemon.RunSuccess("client", "propose-storage-deal", fixtures.TestMiners[0], dataCid, "0", "5").ReadStdoutTrimNewlines()

	splitOnSpace := strings.Split(proposeDealOutput, " ")

	dealCid := splitOnSpace[len(splitOnSpace)-1]

	minerDaemon.Restart()
	minerDaemon.RunSuccess("mining", "start")

	clientDaemon.Restart()

	minerDaemon.ConnectSuccess(clientDaemon)

	assert.NotEmpty(t, clientDaemon.RunSuccess("client", "query-storage-deal", dealCid).ReadStdout())
}

func TestDuplicateDeals(t *testing.T) {
	t.Skip("Long term solution: #3642")
	tf.IntegrationTest(t)

	// Give the deal time to complete
	ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fast.FilecoinOpts{
		InitOpts:   []fast.ProcessInitOption{fast.POAutoSealIntervalSeconds(1)},
		DaemonOpts: []fast.ProcessDaemonOption{fast.POBlockTime(50 * time.Millisecond)},
	})
	defer func() {
		require.NoError(t, env.Teardown(ctx))
	}()
	clientDaemon := env.GenesisMiner

	minerDaemon := env.RequireNewNodeWithFunds(1111)

	duration := uint64(5)
	collateral := big.NewInt(int64(100))
	askPrice := big.NewFloat(0.5)
	expiry := big.NewInt(int64(10000))

	pparams, err := minerDaemon.Protocol(ctx)
	require.NoError(t, err)

	sinfo := pparams.SupportedSectors[0]

	// mine the create storage message, then mine the set ask message
	series.CtxMiningNext(ctx, 2)

	ask, err := series.CreateStorageMinerWithAsk(ctx, minerDaemon, collateral, askPrice, expiry, sinfo.Size)
	require.NoError(t, err)

	err = minerDaemon.MiningSetup(ctx)
	require.NoError(t, err)

	// mine createChannel message
	series.CtxMiningNext(ctx, 1)

	_, err = minerClientMakeDealWithAllowDupes(ctx, t, true, minerDaemon, clientDaemon, ask.ID, duration)
	require.NoError(t, err)

	t.Run("Can make a second deal if --allow-duplicates is passed", func(t *testing.T) {
		// mine createChannel message
		series.CtxMiningNext(ctx, 1)

		dealResp, err := minerClientMakeDealWithAllowDupes(ctx, t, true, minerDaemon, clientDaemon, ask.ID, duration)
		assert.NoError(t, err)
		require.NotNil(t, dealResp)
		assert.Equal(t, storagedeal.Accepted, dealResp.State)
	})
	t.Run("Cannot make a second deal --allow-duplicates is NOT passed", func(t *testing.T) {
		dealResp, err := minerClientMakeDealWithAllowDupes(ctx, t, false, minerDaemon, clientDaemon, ask.ID, duration)
		assert.Error(t, err)
		assert.Nil(t, dealResp)
	})
}

// requireMakeDeal creates a deal with allowDuplicates set to true
func minerClientMakeDealWithAllowDupes(ctx context.Context, t *testing.T, allowDupes bool, minerDaemon, clientDaemon *fast.Filecoin, askID uint64, duration uint64) (*storagedeal.Response, error) {
	f := files.NewBytesFile([]byte("HODLHODLHODL"))
	dataCid, err := clientDaemon.ClientImport(ctx, f)
	require.NoError(t, err)

	var minerAddress address.Address
	err = minerDaemon.ConfigGet(ctx, "mining.minerAddress", &minerAddress)
	require.NoError(t, err)
	dealResponse, err := clientDaemon.ClientProposeStorageDeal(ctx, dataCid, minerAddress, askID, duration, fast.AOAllowDuplicates(allowDupes))
	return dealResponse, err
}

func TestDealWithSameDataAndDifferentMiners(t *testing.T) {
	t.Skip("Long term solution: #3642")
	tf.IntegrationTest(t)

	ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fast.FilecoinOpts{
		InitOpts:   []fast.ProcessInitOption{fast.POAutoSealIntervalSeconds(1)},
		DaemonOpts: []fast.ProcessDaemonOption{fast.POBlockTime(50 * time.Millisecond)},
	})
	defer func() {
		require.NoError(t, env.Teardown(ctx))
	}()
	clientDaemon := env.GenesisMiner

	collateral := big.NewInt(int64(1))
	price := big.NewFloat(float64(0.001))
	expiry := big.NewInt(int64(500))

	// create first miner
	miner1Daemon := env.RequireNewNodeWithFunds(1111)
	require.NoError(t, series.Connect(ctx, miner1Daemon, clientDaemon))
	pparams, err := miner1Daemon.Protocol(ctx)
	require.NoError(t, err)
	sinfo := pparams.SupportedSectors[0]

	// mine the create miner message, then mine the set ask message
	series.CtxMiningNext(ctx, 2)
	ask1, err := series.CreateStorageMinerWithAsk(ctx, miner1Daemon, collateral, price, expiry, sinfo.Size)
	require.NoError(t, miner1Daemon.MiningSetup(ctx))
	require.NoError(t, err)

	// create second miner
	miner2Daemon := env.RequireNewNodeWithFunds(1111)
	require.NoError(t, series.Connect(ctx, miner2Daemon, clientDaemon))

	// mine the create miner message, then mine the set ask message
	series.CtxMiningNext(ctx, 2)
	ask2, err := series.CreateStorageMinerWithAsk(ctx, miner2Daemon, collateral, price, expiry, sinfo.Size)
	require.NoError(t, miner2Daemon.MiningSetup(ctx))
	require.NoError(t, err)

	// define data
	data := []byte("HODLHODLHODL")

	// Make storage deal with first miner (mining the payment channel creation)
	series.CtxMiningNext(ctx, 1)
	_, deal, err := series.ImportAndStore(ctx, clientDaemon, ask1, files.NewBytesFile(data))
	require.NoError(t, err)
	require.Equal(t, storagedeal.Accepted, deal.State)

	// Make storage deal with second miner using same data and assert no error (mining the payment channel creation)
	series.CtxMiningNext(ctx, 1)
	_, deal, err = series.ImportAndStore(ctx, clientDaemon, ask2, files.NewBytesFile(data))
	assert.NoError(t, err)
	assert.Equal(t, storagedeal.Accepted, deal.State)
}

func TestVoucherPersistenceAndPayments(t *testing.T) {
	t.Skip("Long term solution: #3642")
	tf.IntegrationTest(t)

	// DefaultAddress required here
	miner := th.NewDaemon(t,
		th.WithMiner(fixtures.TestMiners[0]),
		th.KeyFile(fixtures.KeyFilePaths()[0]),
		th.DefaultAddress(fixtures.TestAddresses[0]),
	).Start()
	defer miner.ShutdownSuccess()

	client := th.NewDaemon(t, th.KeyFile(fixtures.KeyFilePaths()[2]), th.DefaultAddress(fixtures.TestAddresses[2])).Start()
	defer client.ShutdownSuccess()

	miner.RunSuccess("mining start")
	miner.UpdatePeerID()

	miner.ConnectSuccess(client)

	miner.MinerSetPrice(fixtures.TestMiners[0], fixtures.TestAddresses[0], "20", "10")
	dataCid := client.RunWithStdin(strings.NewReader("HODLHODLHODL"), "client", "import").ReadStdoutTrimNewlines()

	proposeDealOutput := client.RunSuccess("client", "propose-storage-deal", fixtures.TestMiners[0], dataCid, "0", "3000").ReadStdoutTrimNewlines()

	splitOnSpace := strings.Split(proposeDealOutput, " ")

	dealCid := splitOnSpace[len(splitOnSpace)-1]

	result := client.RunSuccess("client", "payments", dealCid).ReadStdoutTrimNewlines()

	assert.Contains(t, result, "Channel\tAmount\tValidAt\tEncoded Voucher")
	// Note: in the assertion below the expiration is four digits, but we're only checking
	// two. This is intentional: the expiry depends on the block at which the vouchers were
	// created, which could be any small number eg 0 or 3. The expiry in each case would
	// be 1000/2000/3000 or 1003/2003/3003. Anyway, it's non-deterministic. So we just check
	// the first couple of digits.
	assert.Contains(t, result, "0\t240000\t10")
	assert.Contains(t, result, "0\t480000\t20")
	assert.Contains(t, result, "0\t720000\t30")
}

func TestPieceRejectionInProposeStorageDeal(t *testing.T) {
	t.Skip("Long term solution: #3642")
	tf.IntegrationTest(t)

	minerDaemon := th.NewDaemon(t,
		th.WithMiner(fixtures.TestMiners[0]),
		th.KeyFile(fixtures.KeyFilePaths()[0]),
		th.DefaultAddress(fixtures.TestAddresses[0]),
		th.AutoSealInterval("1"),
	).Start()
	defer minerDaemon.Shutdown()

	clientDaemon := th.NewDaemon(t,
		th.KeyFile(fixtures.KeyFilePaths()[1]),
		th.DefaultAddress(fixtures.TestAddresses[1]),
	).Start()
	defer clientDaemon.ShutdownSuccess()

	minerDaemon.RunSuccess("mining", "start")
	minerDaemon.UpdatePeerID()

	minerDaemon.ConnectSuccess(clientDaemon)

	addAskCid := minerDaemon.MinerSetPrice(fixtures.TestMiners[0], fixtures.TestAddresses[0], "20", "10")
	clientDaemon.WaitForMessageRequireSuccess(addAskCid)

	dataCid := clientDaemon.RunWithStdin(bytes.NewReader(make([]byte, 3000)), "client", "import").ReadStdoutTrimNewlines()

	proposeDealErrors := clientDaemon.Run("client", "propose-storage-deal", fixtures.TestMiners[0], dataCid, "0", "5").ReadStderr()

	assert.Contains(t, proposeDealErrors, "piece is 3000 bytes but sector size is 1016 bytes")
}

func TestSelfDialStorageGoodError(t *testing.T) {
	t.Skip("Long term solution: #3642")
	tf.IntegrationTest(t)

	// set block time sufficiently high that client can import its piece
	// and generate a commitment before the deal proposing context expires
	ctx, env := fastesting.NewTestEnvironment(context.Background(), t, fast.FilecoinOpts{
		DaemonOpts: []fast.ProcessDaemonOption{fast.POBlockTime(100 * time.Millisecond)},
	})

	// Teardown after test ends.
	defer func() {
		err := env.Teardown(ctx)
		require.NoError(t, err)
	}()

	// Start mining.
	miningNode := env.RequireNewNodeWithFunds(1000)

	collateral := big.NewInt(int64(1))
	price := big.NewFloat(float64(0.001))
	expiry := big.NewInt(int64(500))

	pparams, err := miningNode.Protocol(ctx)
	require.NoError(t, err)

	sinfo := pparams.SupportedSectors[0]

	// mine the create storage message, then mine the set ask message
	series.CtxMiningNext(ctx, 2)

	ask, err := series.CreateStorageMinerWithAsk(ctx, miningNode, collateral, price, expiry, sinfo.Size)
	require.NoError(t, err)

	// Try to make a storage deal with self and fail on self dial.
	f := files.NewBytesFile([]byte("satyamevajayate"))
	_, _, err = series.ImportAndStore(ctx, miningNode, ask, f)
	assert.Error(t, err)
	fastesting.AssertStdErrContains(t, miningNode, "attempting to make storage deal with self")
}
