package commands

import (
	"math/big"
	"strings"
	"testing"

	"github.com/filecoin-project/go-filecoin/fixtures"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
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
	d := th.NewDaemon(t, th.WithMiner(fixtures.TestMiners[0]), th.KeyFile(fixtures.KeyFilePaths()[0])).Start()
	defer d.ShutdownSuccess()

	addr := fixtures.TestMiners[0]

	s := d.RunSuccess("wallet", "balance", addr)
	beforeBalance := parseInt(assert, s.ReadStdout())

	d.RunSuccess("mining", "once")

	s = d.RunSuccess("wallet", "balance", addr)
	afterBalance := parseInt(assert, s.ReadStdout())
	sum := new(big.Int)

	assert.True(sum.Add(beforeBalance, big.NewInt(1000)).Cmp(afterBalance) == 0)
}
