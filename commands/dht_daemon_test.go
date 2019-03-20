package commands_test

import (
	"strings"
	"testing"
	"time"

	ast "gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"

	"github.com/filecoin-project/go-filecoin/fixtures"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
)

func TestDhtFindPeer(t *testing.T) {
	t.Parallel()
	assert := ast.New(t)

	d1 := th.NewDaemon(t).Start()
	defer d1.ShutdownSuccess()

	d2 := th.NewDaemon(t).Start()
	defer d2.ShutdownSuccess()

	d1.ConnectSuccess(d2)

	d2Id := d2.GetID()

	findpeerOutput := d1.RunSuccess("dht", "findpeer", d2Id).ReadStdoutTrimNewlines()

	d2Addr := d2.GetAddresses()[0]

	assert.Contains(d2Addr, findpeerOutput)
}

func TestDhtFindProvs(t *testing.T) {
	t.Parallel()
	assert := ast.New(t)

	d1 := makeTestDaemonWithMinerAndStart(t)
	defer d1.ShutdownSuccess()

	d2 := th.NewDaemon(t,
		th.WithMiner(fixtures.TestMiners[0]),
		th.KeyFile(fixtures.KeyFilePaths()[0]),
		th.KeyFile(fixtures.KeyFilePaths()[1])).Start()
	defer d2.ShutdownSuccess()

	d3 := th.NewDaemon(t,
		th.WithMiner(fixtures.TestMiners[0]),
		th.KeyFile(fixtures.KeyFilePaths()[0]),
		th.KeyFile(fixtures.KeyFilePaths()[2])).Start()
	defer d3.ShutdownSuccess()

	d1.ConnectSuccess(d2)
	d1.ConnectSuccess(d3)
	d2.ConnectSuccess(d3)

	d1.MineAndPropagate(5*time.Second, d2)
	d2.MineAndPropagate(5*time.Second, d1)
	d3.MineAndPropagate(5*time.Second, d1, d2)

	op1 := d1.RunSuccess("chain", "ls")
	results := strings.Split(op1.ReadStdout(), "\n")
	assert.True(len(results) > 1)
	genesisKey := results[0]

	t.Run("basic command succeeds", func(t *testing.T) {
		op1 = d1.Run("dht", "findprovs", genesisKey)
		results = strings.Split(op1.ReadStdoutTrimNewlines(), "\n")
		// all miners should have the genesis block key
		assert.Len(results, 3)
	})

	t.Run("verbose option outputs expected info", func(t *testing.T) {
		op1 = d1.Run("dht", "findprovs", genesisKey, "-v")
		results = strings.Split(op1.ReadStdoutTrimNewlines(), "\n")
		// TODO: what's a good test for this?
		assert.NotEmpty(results)
	})

	t.Run("number of providers option limits the providers found", func(t *testing.T) {
		op1 = d1.Run("dht", "findprovs", genesisKey, "-n", "1")
		results = strings.Split(op1.ReadStdoutTrimNewlines(), "\n")
		assert.Len(results, 1)
	})
}
