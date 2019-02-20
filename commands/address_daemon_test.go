package commands

import (
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/fixtures"
	th "github.com/filecoin-project/go-filecoin/testhelpers"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
)

func TestAddrsNewAndList(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	d := th.NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	addrs := make([]string, 10)
	for i := 0; i < 10; i++ {
		addrs[i] = d.CreateWalletAddr()
	}

	list := d.RunSuccess("wallet", "addrs", "ls").ReadStdout()
	for _, addr := range addrs {
		assert.Contains(list, addr)
	}
}

func TestWalletBalance(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	d := th.NewDaemon(t).Start()
	defer d.ShutdownSuccess()
	addr := d.CreateWalletAddr()

	t.Log("[success] not found, zero")
	balance := d.RunSuccess("wallet", "balance", addr)
	assert.Equal("0", balance.ReadStdoutTrimNewlines())

	t.Log("[success] balance 9999900000")
	balance = d.RunSuccess("wallet", "balance", address.NetworkAddress.String())
	assert.Equal("9999900000", balance.ReadStdoutTrimNewlines())

	t.Log("[success] newly generated one")
	addrNew := d.RunSuccess("wallet addrs new")
	balance = d.RunSuccess("wallet", "balance", addrNew.ReadStdoutTrimNewlines())
	assert.Equal("0", balance.ReadStdoutTrimNewlines())
}

func TestAddrLookupAndUpdate(t *testing.T) {
	assert := assert.New(t)
	d1 := th.NewDaemon(t, th.WithMiner(fixtures.TestMiners[0]), th.KeyFile(fixtures.KeyFilePaths()[1])).Start()
	defer d1.ShutdownSuccess()

	d := th.NewDaemon(t, th.WithMiner(fixtures.TestMiners[0]), th.KeyFile(fixtures.KeyFilePaths()[0])).Start()
	defer d.ShutdownSuccess()

	d1.ConnectSuccess(d)

	addr := fixtures.TestAddresses[0]
	minerAddr := fixtures.TestMiners[0]
	minerPidForUpdate := th.RequireRandomPeerID()

	// capture original, pre-update miner pid
	lookupOutA := th.RunSuccessFirstLine(d, "address", "lookup", minerAddr)

	// Not a miner address, should fail.
	d.RunFail("failed to find", "address", "lookup", addr)

	// update the miner's peer ID
	updateMsg := th.RunSuccessFirstLine(d,
		"miner", "update-peerid",
		"--from", addr,
		"--price", "0",
		"--limit", "300",
		minerAddr,
		minerPidForUpdate.Pretty(),
	)

	// ensure mining happens after update message gets included in mempool
	d1.MineAndPropagate(10*time.Second, d)

	// wait for message to be included in a block
	d.WaitForMessageRequireSuccess(core.MustDecodeCid(updateMsg))

	// use the address lookup command to ensure update happened
	lookupOutB := th.RunSuccessFirstLine(d, "address", "lookup", minerAddr)
	assert.Equal(minerPidForUpdate.Pretty(), lookupOutB)
	assert.NotEqual(lookupOutA, lookupOutB)
}

func TestWalletLoadFromFile(t *testing.T) {
	assert := assert.New(t)

	d := th.NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	for _, p := range fixtures.KeyFilePaths() {
		d.RunSuccess("wallet", "import", p)
	}

	dw := d.RunSuccess("address", "ls").ReadStdoutTrimNewlines()

	for _, addr := range fixtures.TestAddresses {
		// assert we loaded the test address from the file
		assert.Contains(dw, addr)
	}

	// assert default amount of funds were allocated to address during genesis
	wb := d.RunSuccess("wallet", "balance", fixtures.TestAddresses[0]).ReadStdoutTrimNewlines()
	assert.Contains(wb, "10000")
}
