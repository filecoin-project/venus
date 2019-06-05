package commands_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/fixtures"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

func TestAddrsNewAndList(t *testing.T) {
	tf.IntegrationTest(t)

	d := th.NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	addrs := make([]string, 10)
	for i := 0; i < 10; i++ {
		addrs[i] = d.CreateAddress()
	}

	list := d.RunSuccess("address", "ls").ReadStdout()
	for _, addr := range addrs {
		assert.Contains(t, list, addr)
	}
}

func TestWalletBalance(t *testing.T) {
	tf.IntegrationTest(t)

	d := th.NewDaemon(t).Start()
	defer d.ShutdownSuccess()
	addr := d.CreateAddress()

	t.Log("[success] not found, zero")
	balance := d.RunSuccess("wallet", "balance", addr)
	assert.Equal(t, "0", balance.ReadStdoutTrimNewlines())

	t.Log("[success] balance 9999900000")
	balance = d.RunSuccess("wallet", "balance", address.NetworkAddress.String())
	assert.Equal(t, "9999900000", balance.ReadStdoutTrimNewlines())

	t.Log("[success] newly generated one")
	addrNew := d.RunSuccess("address new")
	balance = d.RunSuccess("wallet", "balance", addrNew.ReadStdoutTrimNewlines())
	assert.Equal(t, "0", balance.ReadStdoutTrimNewlines())
}

func TestAddrLookupAndUpdate(t *testing.T) {
	tf.IntegrationTest(t)

	d := makeTestDaemonWithMinerAndStart(t)
	defer d.ShutdownSuccess()

	d1 := th.NewDaemon(t,
		th.WithMiner(fixtures.TestMiners[0]),
		th.KeyFile(fixtures.KeyFilePaths()[0]),
		th.KeyFile(fixtures.KeyFilePaths()[1])).Start()
	defer d1.ShutdownSuccess()

	d1.ConnectSuccess(d)

	addr := fixtures.TestAddresses[0]
	minerAddr := fixtures.TestMiners[0]
	minerPidForUpdate := th.RequireRandomPeerID(t)

	// capture original, pre-update miner pid
	lookupOutA := th.RunSuccessFirstLine(d, "address", "lookup", minerAddr)

	// Not a miner address, should fail.
	d.RunFail("failed to find", "address", "lookup", addr)

	// update the miner's peer ID
	updateMsg := th.RunSuccessFirstLine(d,
		"miner", "update-peerid",
		"--from", addr,
		"--gas-price", "1",
		"--gas-limit", "300",
		minerAddr,
		minerPidForUpdate.Pretty(),
	)

	// ensure mining happens after update message gets included in mempool
	d1.MineAndPropagate(10*time.Second, d)

	// wait for message to be included in a block
	d.WaitForMessageRequireSuccess(core.MustDecodeCid(updateMsg))

	// use the address lookup command to ensure update happened
	lookupOutB := th.RunSuccessFirstLine(d, "address", "lookup", minerAddr)
	assert.Equal(t, minerPidForUpdate.Pretty(), lookupOutB)
	assert.NotEqual(t, lookupOutA, lookupOutB)
}

func TestWalletLoadFromFile(t *testing.T) {
	tf.IntegrationTest(t)

	d := th.NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	for _, p := range fixtures.KeyFilePaths() {
		d.RunSuccess("wallet", "import", p)
	}

	dw := d.RunSuccess("address", "ls").ReadStdoutTrimNewlines()

	for _, addr := range fixtures.TestAddresses {
		// assert we loaded the test address from the file
		assert.Contains(t, dw, addr)
	}

	// assert default amount of funds were allocated to address during genesis
	wb := d.RunSuccess("wallet", "balance", fixtures.TestAddresses[0]).ReadStdoutTrimNewlines()
	assert.Contains(t, wb, "10000")
}

func TestWalletExportImportRoundTrip(t *testing.T) {
	tf.IntegrationTest(t)

	d := th.NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	dw := d.RunSuccess("address", "ls").ReadStdoutTrimNewlines()

	ki := d.RunSuccess("wallet", "export", dw, "--enc=json").ReadStdoutTrimNewlines()

	wf, err := os.Create("walletFileTest")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, os.Remove("walletFileTest"))
	}()
	_, err = wf.WriteString(ki)
	require.NoError(t, err)
	require.NoError(t, wf.Close())

	maybeAddr := d.RunSuccess("wallet", "import", wf.Name()).ReadStdoutTrimNewlines()
	assert.Equal(t, dw, maybeAddr)

}

func TestWalletExportPrivateKeyConsistentDisplay(t *testing.T) {
	tf.IntegrationTest(t)

	d := th.NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	dw := d.RunSuccess("address", "ls").ReadStdoutTrimNewlines()

	exportText := d.RunSuccess("wallet", "export", dw).ReadStdoutTrimNewlines()
	exportTextPrivateKeyLine := strings.Split(exportText, "\n")[1]
	exportTextPrivateKey := strings.Split(exportTextPrivateKeyLine, "\t")[1]

	exportJSON := d.RunSuccess("wallet", "export", dw, "--enc=json").ReadStdoutTrimNewlines()

	assert.Contains(t, exportJSON, exportTextPrivateKey)
}
