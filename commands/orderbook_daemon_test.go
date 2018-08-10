package commands

import (
	"fmt"
	"testing"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testfiles"

	"github.com/stretchr/testify/assert"
)

func TestOrderbookBids(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	d := NewDaemon(t, WalletFile(tf.WalletFilePath()), WalletAddr(testAddress3)).Start()
	defer d.ShutdownSuccess()

	d.CreateWalletAddr()

	for i := 0; i < 10; i++ {
		d.RunSuccess("client", "add-bid", "1", fmt.Sprintf("%d", i),
			"--from", testAddress3)
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

	d := NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	minerAddr := d.CreateMinerAddr()

	for i := 0; i < 10; i++ {
		d.RunSuccess(
			"miner", "add-ask",
			"--from", d.Config().Mining.RewardAddress.String(),
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
	client := NewDaemon(t).Start()
	defer func() { t.Log(client.ReadStderr()) }()
	defer client.ShutdownSuccess()

	// make a miner
	miner := NewDaemon(t).Start()
	defer func() { t.Log(miner.ReadStderr()) }()
	defer miner.ShutdownSuccess()

	// make friends
	client.ConnectSuccess(miner)

	// make a deal
	dealData := "how linked lists will change the world"
	dealDataCid := client.MakeDeal(dealData, miner)

	// both the miner and client can get the deal
	// with the expected cid inside
	cliDealO := client.RunSuccess("orderbook", "deals")
	minDealO := miner.RunSuccess("orderbook", "deals")
	assert.Contains(cliDealO.ReadStdout(), dealDataCid)
	assert.Contains(minDealO.ReadStdout(), dealDataCid)

}
