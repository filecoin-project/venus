package commands

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestBidList(t *testing.T) {
	assert := assert.New(t)

	d := NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	makeAddr(t, d)

	// make 10 bids
	for i := 0; i < 10; i++ {
		d.RunSuccess("client add-bid", "1", fmt.Sprintf("%d", i),
			"--from", core.TestAccount.String(),
		)
	}

	// mine 10 bids
	for i := 0; i < 10; i++ {
		d.RunSuccess("mining once")
	}

	// count 10 bids
	list := d.RunSuccess("orderbook bids").ReadStdout()
	for i := 0; i < 10; i++ {
		assert.Contains(list, fmt.Sprintf("\"price\":%d", i))
	}

}

func makeMinerAddress(d *TestDaemon) types.Address {
	miner := d.RunSuccess("miner create",
		"--from", core.TestAccount.String(), "1000000", "20",
	)
	minerMessageCid, err := cid.Parse(strings.Trim(miner.ReadStdout(), "\n"))
	assert.NoError(d.test, err)

	var wg sync.WaitGroup
	var minerAddr types.Address

	wg.Add(1)
	go func() {
		wait := d.RunSuccess("message wait",
			"--return",
			"--message=false",
			"--receipt=false",
			minerMessageCid.String(),
		)
		addr, err := types.NewAddressFromString(strings.Trim(wait.ReadStdout(), "\n"))
		assert.NoError(d.test, err)
		assert.NotEqual(d.test, addr, types.Address{})
		minerAddr = addr
		wg.Done()
	}()

	d.RunSuccess("mining once")

	wg.Wait()
	return minerAddr
}

func TestAskList(t *testing.T) {
	assert := assert.New(t)

	d := NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	makeAddr(t, d)
	minerAddr := makeMinerAddress(d)

	// make 10 bids
	for i := 0; i < 10; i++ {
		d.RunSuccess("miner add-ask", minerAddr.String(), "1", fmt.Sprintf("%d", i),
			"--from", core.TestAccount.String(),
		)
	}

	// mine 10 bids
	for i := 0; i < 10; i++ {
		d.RunSuccess("mining once")
	}

	// count 10 bids
	list := d.RunSuccess("orderbook asks").ReadStdout()
	for i := 0; i < 10; i++ {
		assert.Contains(list, fmt.Sprintf("\"price\":%d", i))
	}

}

// TODO when PR 190 merges
func TestDealList(t *testing.T) {
}
