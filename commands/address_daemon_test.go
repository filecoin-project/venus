package commands

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"

	"github.com/stretchr/testify/assert"
)

func TestAddrsNewAndList(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	d := NewDaemon(t).Start()
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

	d := NewDaemon(t).Start()
	defer d.ShutdownSuccess()
	addr := d.CreateWalletAddr()

	t.Log("[success] not found, zero")
	balance := d.RunSuccess("wallet", "balance", addr)
	assert.Equal("0", balance.readStdoutTrimNewlines())

	t.Log("[success] balance 10000000")
	balance = d.RunSuccess("wallet", "balance", address.NetworkAddress.String())
	assert.Equal("10000000", balance.readStdoutTrimNewlines())

	t.Log("[success] newly generated one")
	addrNew := d.RunSuccess("wallet addrs new")
	balance = d.RunSuccess("wallet", "balance", addrNew.readStdoutTrimNewlines())
	assert.Equal("0", balance.readStdoutTrimNewlines())
}

func TestAddrsLookup(t *testing.T) {
	assert := assert.New(t)

	d := NewDaemon(t, CmdTimeout(time.Second*90)).Start()
	defer d.ShutdownSuccess()

	var wg sync.WaitGroup
	var minerAddr types.Address

	newMinerPid := core.RequireRandomPeerID()

	// mine to ensure that an account actor is created
	d.RunSuccess("mining", "once")

	// get the account actor's address
	minerOwnerAddr := runSuccessFirstLine(d, "wallet", "addrs", "ls")

	wg.Add(1)
	go func() {
		miner := d.RunSuccess("miner", "create",
			"--from", minerOwnerAddr,
			"--peerid", newMinerPid.Pretty(),
			"1000000", "20",
		)
		addr, err := types.NewAddressFromString(strings.Trim(miner.ReadStdout(), "\n"))
		assert.NoError(err)
		assert.NotEqual(addr, types.Address{})
		minerAddr = addr
		wg.Done()
	}()

	// ensure mining runs after the command in our goroutine
	d.RunSuccess("mpool --wait-for-count=1")

	d.RunSuccess("mining once")

	wg.Wait()

	lookupOut := runSuccessFirstLine(d, "address", "lookup", minerAddr.String())

	assert.Equal(newMinerPid.Pretty(), lookupOut)
}
