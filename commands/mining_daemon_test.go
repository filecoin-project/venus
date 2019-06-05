package commands_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/fixtures"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

func parseInt(t *testing.T, s string) *big.Int {
	i := new(big.Int)
	i, err := i.SetString(strings.TrimSpace(s), 10)
	assert.True(t, err, "couldn't parse as big.Int %q", s)
	return i
}

func TestMiningGenBlock(t *testing.T) {
	tf.IntegrationTest(t)

	d := makeTestDaemonWithMinerAndStart(t)
	defer d.ShutdownSuccess()

	addr := fixtures.TestAddresses[0]

	s := d.RunSuccess("wallet", "balance", addr)
	beforeBalance := parseInt(t, s.ReadStdout())

	d.RunSuccess("mining", "once")

	s = d.RunSuccess("wallet", "balance", addr)
	afterBalance := parseInt(t, s.ReadStdout())
	sum := new(big.Int)

	assert.Equal(t, sum.Add(beforeBalance, big.NewInt(1000)), afterBalance)
}
