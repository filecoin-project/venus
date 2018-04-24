package commands

import (
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/core"
)

func TestClientAddBidSuccess(t *testing.T) {
	assert := assert.New(t)

	d := NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	d.CreateWalletAddr()

	bid := d.RunSuccess("client", "add-bid", "2000", "10",
		"--from", core.TestAddress.String(),
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
	d.CreateWalletAddr()

	d.RunFail(
		"invalid from address",
		"client", "add-bid", "2000", "10",
		"--from", "hello",
	)
	d.RunFail(
		"invalid size",
		"client", "add-bid", "2f", "10",
		"--from", core.TestAddress.String(),
	)
	d.RunFail(
		"invalid price",
		"client", "add-bid", "10", "3f",
		"--from", core.TestAddress.String(),
	)
}

func TestProposeDeal(t *testing.T) {
	dcli := NewDaemon(t).Start()
	defer func() { t.Log(dcli.ReadStderr()) }()
	defer dcli.ShutdownSuccess()
	dmin := NewDaemon(t).Start()
	defer func() { t.Log(dmin.ReadStderr()) }()
	defer dmin.ShutdownSuccess()

	dcli.ConnectSuccess(dmin)

	// mine here to get some moneys
	dcli.RunSuccess("mining", "once")
	time.Sleep(time.Millisecond * 20)
	dcli.RunSuccess("mining", "once")
	time.Sleep(time.Millisecond * 20)
	dmin.RunSuccess("mining", "once")
	time.Sleep(time.Millisecond * 20)
	dmin.RunSuccess("mining", "once")
	time.Sleep(time.Millisecond * 20)

	miner := dmin.CreateMinerAddr()

	askO := dmin.RunSuccess(
		"miner", "add-ask",
		"--from", dmin.Config().Mining.RewardAddress.String(),
		miner.String(), "1200", "1",
	)
	dmin.RunSuccess("mining", "once")
	dmin.RunSuccess("message", "wait", "--return", strings.TrimSpace(askO.ReadStdout()))

	dcli.RunSuccess(
		"client", "add-bid",
		"--from", dcli.Config().Mining.RewardAddress.String(),
		"500", "1",
	)
	dcli.RunSuccess("mining", "once")
	time.Sleep(time.Millisecond * 20) // wait for block propagation

	buf := strings.NewReader("filecoin is a blockchain")
	o := dcli.RunWithStdin(buf, "client", "import").AssertSuccess()
	data := strings.TrimSpace(o.ReadStdout())

	negidO := dcli.RunSuccess("client", "propose-deal", "--ask=0", "--bid=0", data)

	time.Sleep(time.Millisecond * 20)
	dmin.RunSuccess("mining", "once")

	negid := strings.Split(strings.Split(negidO.ReadStdout(), "\n")[1], " ")[1]
	dcli.RunSuccess("client", "query-deal", negid)

	// TODO: this command doesnt quite work
	//dealO := dcli.RunSuccess("orderbook", "deals")
	//t.Fatal(dealO.ReadStdout())
}
