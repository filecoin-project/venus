package commands_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/filecoin-project/go-filecoin/fixtures"
	"github.com/filecoin-project/go-filecoin/protocol/storage"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
)

func TestListAsks(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	minerDaemon := makeTestDaemonWithMinerAndStart(t)
	defer minerDaemon.ShutdownSuccess()

	minerDaemon.RunSuccess("mining start")
	minerDaemon.MinerSetPrice(fixtures.TestMiners[0], fixtures.TestAddresses[0], "20", "10")

	listAsksOutput := minerDaemon.RunSuccess("client", "list-asks").ReadStdoutTrimNewlines()
	assert.Equal(fixtures.TestMiners[0]+" 000 20 11", listAsksOutput)
}

func TestStorageDealsAfterRestart(t *testing.T) {
	assert := assert.New(t)
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

	assert.NotEmpty(clientDaemon.RunSuccess("client", "query-storage-deal", dealCid).ReadStdout())
}

func TestDuplicateDeals(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

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

	client.RunSuccess("client", "propose-storage-deal", fixtures.TestMiners[0], dataCid, "0", "5")

	t.Run("propose a duplicate deal with the '--allow-duplicates' flag", func(t *testing.T) {
		client.RunSuccess("client", "propose-storage-deal", "--allow-duplicates", fixtures.TestMiners[0], dataCid, "0", "5")
		client.RunSuccess("client", "propose-storage-deal", "--allow-duplicates", fixtures.TestMiners[0], dataCid, "0", "5")
	})

	t.Run("propose a duplicate deal _WITHOUT_ the '--allow-duplicates' flag", func(t *testing.T) {
		proposeDealOutput := client.Run("client", "propose-storage-deal", fixtures.TestMiners[0], dataCid, "0", "5").ReadStderr()
		expectedError := fmt.Sprintf("Error: %s", storage.Errors[storage.ErrDuplicateDeal].Error())
		assert.Equal(expectedError, proposeDealOutput)
	})
}

func TestDealWithSameDataAndDifferentMiners(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

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

	miner2Addr := miner2.CreateMinerAddr(miner1, minerOwner2)
	miner2.UpdatePeerID()

	miner2.RunSuccess("mining start")

	miner1.MinerSetPrice(miner1Addr, minerOwner1, "20", "10")
	miner2.MinerSetPrice(miner2Addr.String(), minerOwner2, "20", "10")

	dataCid := client.RunWithStdin(strings.NewReader("HODLHODLHODL"), "client", "import").ReadStdoutTrimNewlines()

	firstDeal := client.RunSuccess("client", "propose-storage-deal", miner1Addr, dataCid, "0", "5").ReadStdoutTrimNewlines()
	assert.Contains(firstDeal, "accepted")
	secondDeal := client.RunSuccess("client", "propose-storage-deal", miner2Addr.String(), dataCid, "0", "5").ReadStdoutTrimNewlines()
	assert.Contains(secondDeal, "accepted")
}

func TestVoucherPersistenceAndPayments(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

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

	assert.Contains(result, "Channel\tAmount\tValidAt\tEncoded Voucher")
	// Note: in the assertion below the expiration is four digits, but we're only checking
	// two. This is intentional: the expiry depends on the block at which the vouchers were
	// created, which could be any small number eg 0 or 3. The expiry in each case would
	// be 1000/2000/3000 or 1003/2003/3003. Anyway, it's non-deterministic. So we just check
	// the first couple of digits.
	assert.Contains(result, "0\t240000\t10")
	assert.Contains(result, "0\t480000\t20")
	assert.Contains(result, "0\t720000\t30")
}
