package commands_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/fixtures"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

func TestDealsRedeem(t *testing.T) {
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

	walletBalanceOutput := minerDaemon.RunSuccess("wallet", "balance", fixtures.TestAddresses[0]).ReadStdoutTrimNewlines()
	oldWalletBalance, err := strconv.ParseFloat(walletBalanceOutput, 64)
	require.NoError(t, err)

	redeemCid := minerDaemon.RunSuccess("deals", "redeem", dealCid, "--gas-price", "1", "--gas-limit", "100").ReadStdoutTrimNewlines()
	minerDaemon.RunSuccess("message", "wait", redeemCid)

	walletBalanceOutput = minerDaemon.RunSuccess("wallet", "balance", fixtures.TestAddresses[0]).ReadStdoutTrimNewlines()
	newWalletBalance, err := strconv.ParseFloat(walletBalanceOutput, 64)
	require.NoError(t, err)

	assert.Equal(t, float64(1000), (newWalletBalance - oldWalletBalance))
}
