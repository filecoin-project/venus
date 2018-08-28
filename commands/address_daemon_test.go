package commands

import (
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	th "github.com/filecoin-project/go-filecoin/testhelpers"

	"github.com/stretchr/testify/assert"
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

	t.Log("[success] balance 10000000000")
	balance = d.RunSuccess("wallet", "balance", address.NetworkAddress.String())
	assert.Equal("10000000000", balance.ReadStdoutTrimNewlines())

	t.Log("[success] newly generated one")
	addrNew := d.RunSuccess("wallet addrs new")
	balance = d.RunSuccess("wallet", "balance", addrNew.ReadStdoutTrimNewlines())
	assert.Equal("0", balance.ReadStdoutTrimNewlines())
}

func TestAddrLookupAndUpdate(t *testing.T) {
	assert := assert.New(t)
	d := th.NewDaemon(t, th.CmdTimeout(time.Second*90)).Start()
	defer d.ShutdownSuccess()

	addr := d.GetDefaultAddress()
	minerPidForUpdate := core.RequireRandomPeerID()

	minerAddr := d.CreateMinerAddr(addr)

	// capture original, pre-update miner pid
	lookupOutA := th.RunSuccessFirstLine(d, "address", "lookup", minerAddr.String())

	// update the miner's peer ID
	updateMsg := th.RunSuccessFirstLine(d,
		"miner", "update-peerid",
		"--from", addr,
		minerAddr.String(),
		minerPidForUpdate.Pretty(),
	)

	// ensure mining happens after update message gets included in mempool
	d.RunSuccess("mpool --wait-for-count=1")
	d.RunSuccess("mining once")

	// wait for message to be included in a block
	d.WaitForMessageRequireSuccess(core.MustDecodeCid(updateMsg))

	// use the address lookup command to ensure update happened
	lookupOutB := th.RunSuccessFirstLine(d, "address", "lookup", minerAddr.String())
	assert.Equal(minerPidForUpdate.Pretty(), lookupOutB)
	assert.NotEqual(lookupOutA, lookupOutB)
}

func TestWalletLoadFromFile(t *testing.T) {
	t.Skip("FIXME: this should use wallet import now instead of relying on initialization")
	assert := assert.New(t)

	d := th.NewDaemon(t, th.WalletFile("../testhelpers/testfiles/walletGenFile.toml"), th.WalletAddr("fcqrn3nwxlpqng6ms8kp4tk44zrjyh4nurrmg6wth")).Start()
	defer d.ShutdownSuccess()

	// assert we loaded the test address from the file
	dw := d.RunSuccess("address", "ls").ReadStdoutTrimNewlines()
	assert.Contains(dw, "fcqrn3nwxlpqng6ms8kp4tk44zrjyh4nurrmg6wth")

	// assert default amount of funds were allocated to address during genesis
	wb := d.RunSuccess("wallet", "balance", "fcqrn3nwxlpqng6ms8kp4tk44zrjyh4nurrmg6wth").ReadStdoutTrimNewlines()
	assert.Contains(wb, "10000000")
}
