package commands

import (
	"fmt"
	"testing"

	th "github.com/filecoin-project/go-filecoin/testhelpers"

	"github.com/stretchr/testify/assert"
)

func TestOrderbookBids(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	d := th.NewDaemon(t, th.KeyFile(th.TestKey3), th.WalletAddr(th.TestAddress3)).Start()
	defer d.ShutdownSuccess()

	d.CreateWalletAddr()

	for i := 0; i < 10; i++ {
		d.RunSuccess("client", "add-bid", "1", fmt.Sprintf("%d", i),
			"--from", th.TestAddress3)
	}

	for i := 0; i < 10; i++ {
		d.RunSuccess("mining", "once")
	}

	list := d.RunSuccess("orderbook", "bids").ReadStdout()
	for i := 0; i < 10; i++ {
		assert.Contains(list, fmt.Sprintf("\"price\":\"%d\"", i))
	}
}

func TestOrderbookAsks(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	d := th.NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	addr := d.GetDefaultAddress()

	minerAddr := d.CreateMinerAddr(addr)

	for i := 0; i < 10; i++ {
		d.RunSuccess(
			"miner", "add-ask",
			"--from", addr,
			minerAddr.String(), "1", fmt.Sprintf("%d", i),
		)
	}

	d.RunSuccess("mining", "once")

	list := d.RunSuccess("orderbook", "asks").ReadStdout()
	for i := 0; i < 10; i++ {
		assert.Contains(list, fmt.Sprintf("\"price\":\"%d\"", i))
	}

}
