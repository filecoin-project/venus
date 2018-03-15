package commands

import (
	"strings"
	"testing"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
)

func makeAddr(t *testing.T, d *TestDaemon) string {
	t.Helper()
	outNew := d.RunSuccess("wallet addrs new")
	addr := strings.Trim(outNew.ReadStdout(), "\n")
	assert.NotEmpty(t, addr)
	return addr
}

func TestAddrsNewAndList(t *testing.T) {
	assert := assert.New(t)

	d := NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	addrs := make([]string, 10)
	for i := 0; i < 10; i++ {
		addrs[i] = makeAddr(t, d)
	}

	list := d.RunSuccess("wallet addrs list").ReadStdout()
	for _, addr := range addrs {
		assert.Contains(list, addr)
	}
}

func TestWalletBalance(t *testing.T) {
	assert := assert.New(t)

	d := NewDaemon(t).Start()
	defer d.ShutdownSuccess()
	addr := makeAddr(t, d)

	t.Log("[failure] not found")
	d.RunFail("not found", "wallet balance", addr)

	t.Log("[success] balance 100000")
	balance := d.RunSuccess("wallet balance", types.Address("filecoin").String())
	assert.Contains(balance.ReadStdout(), "100000")
}
