package commands

import (
	"math/big"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func parseInt(assert *assert.Assertions, s string) *big.Int {
	i := new(big.Int)
	i, err := i.SetString(strings.TrimSpace(s), 10)
	assert.True(err, "couldn't parse as big.Int %q", s)
	return i
}

func TestMinerGenBlock(t *testing.T) {
	assert := assert.New(t)
	d := NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	// TODO: uncomment when we drop the test address
	// we have an address as long as we have the test address setup
	// t.Log("[failure] no addresses")
	// d.RunFail("no addresses in wallet to mine", "mining once")

	t.Log("[success] address in local wallet")
	// Mining reward accrues to first address in the wallet (for now).
	addr := strings.TrimSpace(d.RunSuccess("wallet", "addrs", "list").ReadStdout())
	// TODO: currently, running 'wallet balance' on an address with no funds
	// results in an 'Error: not found'. This needs to be changed to just
	// return a zero balance. When that happens, remove the following line, and
	// uncomment the two after it
	beforeBalance := big.NewInt(0)
	//s := d.RunSuccess("wallet", "balance", addr)
	//beforeBalance := parseInt(assert, s.ReadStdout())
	d.RunSuccess("mining", "once")
	s := d.RunSuccess("wallet", "balance", addr)
	afterBalance := parseInt(assert, s.ReadStdout())
	sum := new(big.Int)
	assert.True(sum.Add(beforeBalance, big.NewInt(1000)).Cmp(afterBalance) == 0)
}
