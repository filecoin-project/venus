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

	d := th.NewDaemon(t, th.WalletAddr(th.TestAddress3)).Start()
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

	minerAddr := d.CreateMinerAddr(th.TestAddress1)

	for i := 0; i < 10; i++ {
		d.RunSuccess(
			"miner", "add-ask",
			minerAddr.String(), "1", fmt.Sprintf("%d", i),
		)
	}

	d.RunSuccess("mining", "once")

	list := d.RunSuccess("orderbook", "asks").ReadStdout()
	for i := 0; i < 10; i++ {
		assert.Contains(list, fmt.Sprintf("\"price\":\"%d\"", i))
	}

}

func TestOrderbookDeals(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	// make a client
	client := th.NewDaemon(t).Start()
	defer func() { t.Log(client.ReadStderr()) }()
	defer client.ShutdownSuccess()

	// make a miner
	miner := th.NewDaemon(t).Start()
	defer func() { t.Log(miner.ReadStderr()) }()
	defer miner.ShutdownSuccess()

	// make friends
	client.ConnectSuccess(miner)

	// make a deal
	dealData := "how linked lists will change the world"
	dealDataCid := client.MakeDeal(dealData, miner, th.TestAddress1)

	// both the miner and client can get the deal
	// with the expected cid inside
	cliDealO := client.RunSuccess("orderbook", "deals")
	minDealO := miner.RunSuccess("orderbook", "deals")
	assert.Contains(cliDealO.ReadStdout(), dealDataCid)
	assert.Contains(minDealO.ReadStdout(), dealDataCid)

}
