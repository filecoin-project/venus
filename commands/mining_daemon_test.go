package commands

import (
	"math/big"
	"strings"
	"testing"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/stretchr/testify/assert"
)

func parseInt(assert *assert.Assertions, s string) *big.Int {
	i := new(big.Int)
	i, err := i.SetString(strings.TrimSpace(s), 10)
	assert.True(err, "couldn't parse as big.Int %q", s)
	return i
}

func TestMiningGenBlock(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	d := th.NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	t.Log("[success] address in local wallet")
	// TODO: use `config` cmd once it exists
	addr := d.Config().Mining.RewardAddress.String()

	s := d.RunSuccess("wallet", "balance", addr)
	beforeBalance := parseInt(assert, s.ReadStdout())
	d.RunSuccess("mining", "once")
	s = d.RunSuccess("wallet", "balance", addr)
	afterBalance := parseInt(assert, s.ReadStdout())
	sum := new(big.Int)
	assert.True(sum.Add(beforeBalance, big.NewInt(1000)).Cmp(afterBalance) == 0)
}
