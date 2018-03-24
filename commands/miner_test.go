package commands

import (
	"strings"
	"sync"
	"testing"

	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestMinerCreateSuccess(t *testing.T) {
	assert := assert.New(t)

	d := NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	d.CreateWalletAdder()

	miner := d.RunSuccess("miner", "create",
		"--from", core.TestAccount.String(), "1000000", "20",
	)
	minerMessageCid, err := cid.Parse(strings.Trim(miner.ReadStdout(), "\n"))
	require.NoError(t, err)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		wait := d.RunSuccess("message", "wait",
			"--return",
			"--message=false",
			"--receipt=false",
			minerMessageCid.String(),
		)
		addr, err := types.NewAddressFromString(strings.Trim(wait.ReadStdout(), "\n"))
		assert.NoError(err)
		assert.NotEqual(addr, types.Address{})
		wg.Done()
	}()

	d.RunSuccess("mining once")

	wg.Wait()
}

func TestMinerCreateFail(t *testing.T) {

	d := NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	d.CreateWalletAdder()

	d.RunFail("invalid from address",
		"miner", "create",
		"--from", "hello", "1000000", "20",
	)
	d.RunFail("invalid pledge",
		"miner", "create",
		"--from", core.TestAccount.String(), "'-123'", "20",
	)
	d.RunFail("invalid pledge",
		"miner", "create",
		"--from", core.TestAccount.String(), "1f", "20",
	)
	d.RunFail("invalid collateral",
		"miner", "create",
		"--from", core.TestAccount.String(), "100", "2f",
	)

}

func TestMinerAddAskSuccess(t *testing.T) {
	assert := assert.New(t)

	d := NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	d.CreateWalletAdder()

	miner := d.RunSuccess(
		"miner", "create",
		"--from", core.TestAccount.String(), "1000000", "20",
	)
	minerMessageCid, err := cid.Parse(strings.Trim(miner.ReadStdout(), "\n"))
	require.NoError(t, err)

	var wg sync.WaitGroup
	var minerAddr types.Address

	wg.Add(1)
	go func() {
		wait := d.RunSuccess(
			"message", "wait",
			"--return",
			"--message=false",
			"--receipt=false",
			minerMessageCid.String(),
		)
		addr, err := types.NewAddressFromString(strings.Trim(wait.ReadStdout(), "\n"))
		assert.NoError(err)
		assert.NotEqual(addr, types.Address{})
		minerAddr = addr
		wg.Done()
	}()

	d.RunSuccess("mining once")

	wg.Wait()

	wg.Add(1)
	go func() {
		ask := d.RunSuccess("miner", "add-ask", minerAddr.String(), "2000", "10",
			"--from", core.TestAccount.String(),
		)
		askCid, err := cid.Parse(strings.Trim(ask.ReadStdout(), "\n"))
		require.NoError(t, err)
		assert.NotNil(askCid)

		wg.Done()
	}()

	d.RunSuccess("mining once")

	wg.Wait()
}

func TestMinerAddAskFail(t *testing.T) {
	assert := assert.New(t)

	d := NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	d.CreateWalletAdder()

	miner := d.RunSuccess("miner", "create",
		"--from", core.TestAccount.String(), "1000000", "20",
	)
	minerMessageCid, err := cid.Parse(strings.Trim(miner.ReadStdout(), "\n"))
	require.NoError(t, err)

	var wg sync.WaitGroup
	var minerAddr types.Address

	wg.Add(1)
	go func() {
		wait := d.RunSuccess("message", "wait",
			"--return",
			"--message=false",
			"--receipt=false",
			minerMessageCid.String(),
		)
		addr, err := types.NewAddressFromString(strings.Trim(wait.ReadStdout(), "\n"))
		assert.NoError(err)
		assert.NotEqual(addr, types.Address{})
		minerAddr = addr
		wg.Done()
	}()

	d.RunSuccess("mining once")

	wg.Wait()

	d.RunFail(
		"invalid from address",
		"miner", "add-ask", minerAddr.String(), "2000", "10",
		"--from", "hello",
	)
	d.RunFail(
		"invalid miner address",
		"miner", "add-ask", "hello", "2000", "10",
		"--from", core.TestAccount.String(),
	)
	d.RunFail(
		"invalid size",
		"miner", "add-ask", minerAddr.String(), "2f", "10",
		"--from", core.TestAccount.String(),
	)
	d.RunFail(
		"invalid price",
		"miner", "add-ask", minerAddr.String(), "10", "3f",
		"--from", core.TestAccount.String(),
	)
}
