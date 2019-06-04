package commands_test

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"

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

func TestListDeals(t *testing.T) {
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

	addAskCid := minerDaemon.MinerSetPrice(fixtures.TestMiners[0], fixtures.TestAddresses[0], "20", "10")
	clientDaemon.WaitForMessageRequireSuccess(addAskCid)
	dataCid := clientDaemon.RunWithStdin(strings.NewReader("HODLHODLHODL"), "client", "import").ReadStdoutTrimNewlines()

	proposeDealOutput := clientDaemon.RunSuccess("client", "propose-storage-deal", fixtures.TestMiners[0], dataCid, "0", "5").ReadStdoutTrimNewlines()

	splitOnSpace := strings.Split(proposeDealOutput, " ")
	dealCid := splitOnSpace[len(splitOnSpace)-1]

	expectedClientOutput := fmt.Sprintf(`{
	"minerAddress": "%s",
	"pieceCid": {
		"/": "QmbHmUVAkqZjQXgifDady7m5cYprX1fgtGaTYxUBBTX3At"
	},
	"proposalCid": {
		"/": "%s"
	},
	"state": 2
}`, fixtures.TestMiners[0], dealCid)

	expectedMinerOutput := fmt.Sprintf(`{
	"minerAddress": "%s",
	"pieceCid": {
		"/": "QmbHmUVAkqZjQXgifDady7m5cYprX1fgtGaTYxUBBTX3At"
	},
	"proposalCid": {
		"/": "%s"
	},
	"state": 7
}`, fixtures.TestMiners[0], dealCid)

	t.Run("with no filters", func(t *testing.T) {
		listClientDealsOutput := clientDaemon.RunSuccess("deals", "list").ReadStdoutTrimNewlines()
		assert.Equal(t, expectedClientOutput, listClientDealsOutput)

		listMinerDealsOutput := minerDaemon.RunSuccess("deals", "list").ReadStdoutTrimNewlines()
		assert.Equal(t, expectedMinerOutput, listMinerDealsOutput)
	})

	t.Run("with --miner", func(t *testing.T) {
		listClientDealsOutput := clientDaemon.RunSuccess("deals", "list", "--miner").ReadStdoutTrimNewlines()
		assert.Equal(t, "", listClientDealsOutput)

		listMinerDealsOutput := minerDaemon.RunSuccess("deals", "list", "--miner").ReadStdoutTrimNewlines()
		assert.Equal(t, expectedMinerOutput, listMinerDealsOutput)
	})

	t.Run("with --client", func(t *testing.T) {
		listClientDealsOutput := clientDaemon.RunSuccess("deals", "list", "--client").ReadStdoutTrimNewlines()
		assert.Equal(t, expectedClientOutput, listClientDealsOutput)

		listMinerDealsOutput := minerDaemon.RunSuccess("deals", "list", "--client").ReadStdoutTrimNewlines()
		assert.Equal(t, "", listMinerDealsOutput)
	})
}
