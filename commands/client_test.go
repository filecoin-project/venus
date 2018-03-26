package commands

import (
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"math/big"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/core"
)

func TestClientAddBidSuccess(t *testing.T) {
	assert := assert.New(t)

	d := NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	d.CreateWalletAdder()

	bid := d.RunSuccess("client", "add-bid", "2000", "10",
		"--from", core.TestAccount.String(),
	)
	bidMessageCid, err := cid.Parse(strings.Trim(bid.ReadStdout(), "\n"))
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		wait := d.RunSuccess("message", "wait",
			"--return",
			"--message=false",
			"--receipt=false",
			bidMessageCid.String(),
		)
		out := wait.ReadStdout()
		bidID, ok := new(big.Int).SetString(strings.Trim(out, "\n"), 10)
		assert.True(ok)
		assert.NotNil(bidID)
		wg.Done()
	}()

	d.RunSuccess("mining once")

	wg.Wait()
}

func TestClientAddBidFail(t *testing.T) {
	d := NewDaemon(t).Start()
	defer d.ShutdownSuccess()
	d.CreateWalletAdder()

	d.RunFail(
		"invalid from address",
		"client", "add-bid", "2000", "10",
		"--from", "hello",
	)
	d.RunFail(
		"invalid size",
		"client", "add-bid", "2f", "10",
		"--from", core.TestAccount.String(),
	)
	d.RunFail(
		"invalid price",
		"client", "add-bid", "10", "3f",
		"--from", core.TestAccount.String(),
	)
}
