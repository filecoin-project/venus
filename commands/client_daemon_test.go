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

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/fixtures"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/fastesting"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
)

func TestListAsks(t *testing.T) {
	tf.IntegrationTest(t)

	minerDaemon := makeTestDaemonWithMinerAndStart(t)
	defer minerDaemon.ShutdownSuccess()

	minerDaemon.RunSuccess("mining start")
	minerDaemon.MinerSetPrice(fixtures.TestMiners[0], fixtures.TestAddresses[0], "20", "10")

	listAsksOutput := minerDaemon.RunSuccess("client", "list-asks").ReadStdoutTrimNewlines()
	assert.Equal(t, fixtures.TestMiners[0]+" 000 20 11", listAsksOutput)
}

func TestStorageDealsAfterRestart(t *testing.T) {
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
	require.NoError(t, clientDaemon.MiningStart(ctx))
	defer func() {
		require.NoError(t, clientDaemon.MiningStop(ctx))
	}()

	minerDaemon := env.RequireNewNodeWithFunds(1111)

	duration := uint64(5)
	collateral := big.NewInt(int64(100))
	askPrice := big.NewFloat(0.5)
	expiry := big.NewInt(int64(10000))

	ask, err := series.CreateStorageMinerWithAsk(ctx, minerDaemon, collateral, askPrice, expiry)
	require.NoError(t, err)

	_, err = minerClientMakeDealWithAllowDupes(ctx, t, true, minerDaemon, clientDaemon, ask.ID, duration)
	require.NoError(t, err)

	t.Run("Can make a second deal if --allow-duplicates is passed", func(t *testing.T) {
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
	tf.IntegrationTest(t)

	miner1Addr := fixtures.TestMiners[0]
	minerOwner1 := fixtures.TestAddresses[0]
	miner1 := th.NewDaemon(t,
		th.WithMiner(miner1Addr),
		th.KeyFile(fixtures.KeyFilePaths()[0]),
		th.DefaultAddress(minerOwner1),
	).Start()
	defer miner1.ShutdownSuccess()

	minerOwner2 := fixtures.TestAddresses[1]
	miner2 := th.NewDaemon(t,
		th.KeyFile(fixtures.KeyFilePaths()[1]),
		th.DefaultAddress(minerOwner2),
	).Start()
	defer miner2.ShutdownSuccess()

	client := th.NewDaemon(t, th.KeyFile(fixtures.KeyFilePaths()[2]), th.DefaultAddress(fixtures.TestAddresses[2])).Start()
	defer client.ShutdownSuccess()

	miner1.RunSuccess("mining start")
	miner1.UpdatePeerID()

	miner1.ConnectSuccess(client)
	miner2.ConnectSuccess(client)

	miner2Addr := miner2.CreateStorageMinerAddr(miner1, minerOwner2)
	miner2.UpdatePeerID()

	miner2.RunSuccess("mining start")

	miner1.MinerSetPrice(miner1Addr, minerOwner1, "20", "10")
	miner2.MinerSetPrice(miner2Addr.String(), minerOwner2, "20", "10")

	dataCid := client.RunWithStdin(strings.NewReader("HODLHODLHODL"), "client", "import").ReadStdoutTrimNewlines()

	firstDeal := client.RunSuccess("client", "propose-storage-deal", miner1Addr, dataCid, "0", "5").ReadStdoutTrimNewlines()
	assert.Contains(t, firstDeal, "accepted")
	secondDeal := client.RunSuccess("client", "propose-storage-deal", miner2Addr.String(), dataCid, "0", "5").ReadStdoutTrimNewlines()
	assert.Contains(t, secondDeal, "accepted")
}

func TestVoucherPersistenceAndPayments(t *testing.T) {
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

	// This is what mining start should do, but FAST uses mining once
	// for some very helpful builtins and because of issue 2579 we need to
	// mine once in a loop instead of calling start.  Once #2579 is fixed
	// this can be replaced with start.
	minerCreateDoneCh := make(chan struct{})
	go func() {
		for {
			select {
			case <-minerCreateDoneCh:
				return
			default:
				series.CtxMiningOnce(ctx)
			}
		}
	}()

	collateral := big.NewInt(int64(1))
	price := big.NewFloat(float64(0.001))
	expiry := big.NewInt(int64(500))
	ask, err := series.CreateStorageMinerWithAsk(ctx, miningNode, collateral, price, expiry)
	minerCreateDoneCh <- struct{}{}
	require.NoError(t, err)

	// Try to make a storage deal with self and fail on self dial.
	f := files.NewBytesFile([]byte("satyamevajayate"))
	_, _, err = series.ImportAndStore(ctx, miningNode, ask, f)
	assert.Error(t, err)
	fastesting.AssertStdErrContains(t, miningNode, "attempting to make storage deal with self")
}
