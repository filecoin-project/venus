package commands

import (
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/fixtures"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
)

func TestClientAddBidSuccess(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	d := th.NewDaemon(
		t,
		th.WithMiner(fixtures.TestMiners[0]),
		th.KeyFile(fixtures.KeyFilePaths()[0]),
		th.KeyFile(fixtures.KeyFilePaths()[1]),
	).Start()
	defer d.ShutdownSuccess()

	bid := d.RunSuccess("client", "add-bid", "2000", "10",
		"--from", fixtures.TestAddresses[1],
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
	t.Parallel()

	d := th.NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	d.RunFail(
		"invalid from address",
		"client", "add-bid", "2000", "10",
		"--from", "hello",
	)
	d.RunFail(
		"invalid size",
		"client", "add-bid", "2f", "10",
	)
	d.RunFail(
		"invalid price",
		"client", "add-bid", "10", "3f",
	)
}

func TestProposeDeal(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	dcli := th.NewDaemon(t, th.WithMiner(fixtures.TestMiners[0]), th.KeyFile(fixtures.KeyFilePaths()[1])).Start()
	defer dcli.ShutdownSuccess()

	dmin := th.NewDaemon(t, th.WithMiner(fixtures.TestMiners[0]), th.KeyFile(fixtures.KeyFilePaths()[0])).Start()
	defer dmin.ShutdownSuccess()

	dcli.ConnectSuccess(dmin)

	// max amt of time we'll wait for block propagation
	maxWait := time.Second * 1

	// set the miner to have the peerid of the miner node
	peerID := dmin.GetID()
	dmin.RunSuccess("miner", "update-peerid", "--from", fixtures.TestAddresses[0], fixtures.TestMiners[0], peerID)
	dmin.MineAndPropagate(maxWait, dcli)

	askO := dmin.RunSuccess(
		"miner", "add-ask",
		"--from", fixtures.TestAddresses[0],
		fixtures.TestMiners[0], "1200", "1",
	)
	dmin.MineAndPropagate(maxWait, dcli)
	dmin.RunSuccess("message", "wait", "--return", strings.TrimSpace(askO.ReadStdout()))

	dcli.RunSuccess(
		"client", "add-bid",
		"--from", fixtures.TestAddresses[1],
		"500", "1",
	)
	dcli.MineAndPropagate(maxWait, dmin)

	buf := strings.NewReader("filecoin is a blockchain")
	o := dcli.RunWithStdin(buf, "client", "import").AssertSuccess()
	data := strings.TrimSpace(o.ReadStdout())

	negidO := dcli.RunSuccess("client", "propose-deal", "--from", fixtures.TestAddresses[1], "--ask=0", "--bid=0", data)
	msgs := strings.Split(negidO.ReadStdout(), "\n")

	assert.Equal(strings.Split(msgs[0], " ")[1], "accepted")
	negid := strings.Split(msgs[2], " ")[1]

	dcli.RunSuccess("client", "query-deal", negid)
}
