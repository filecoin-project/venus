package commands

import (
	"fmt"
	"strings"
	"testing"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
)

// makeAddr must be run inside a `withDaemon` context to have access to
// the default daemon.
func makeAddr(t *testing.T) string {
	t.Helper()
	outNew := runSuccess(t, "go-filecoin wallet addrs new")
	addr := strings.Trim(outNew.ReadStdout(), "\n")
	assert.NotEmpty(t, addr)
	return addr
}

func TestAddrsNewAndList(t *testing.T) {
	assert := assert.New(t)

	daemon := withDaemon(func() {
		addrs := make([]string, 10)
		for i := 0; i < 10; i++ {
			addrs[i] = makeAddr(t)
		}

		outList := runSuccess(t, "go-filecoin wallet addrs list")
		list := outList.ReadStdout()

		for _, addr := range addrs {
			assert.Contains(list, addr)
		}
	})
	assert.NoError(daemon.Error)
	assert.Equal(daemon.Code, 0)
}

func TestWalletBalance(t *testing.T) {
	assert := assert.New(t)

	daemon := withDaemon(func() {
		addr := makeAddr(t)

		t.Log("[failure] not found")
		balance := run(fmt.Sprintf("go-filecoin wallet balance %s", addr))
		assert.Contains(balance.ReadStderr(), "not found")
		assert.Equal(balance.Code, 1)
		assert.Empty(balance.ReadStdout())

		t.Log("[success] balance 100000")
		balance = runSuccess(t, fmt.Sprintf("go-filecoin wallet balance %s", types.Address("filecoin")))
		assert.Contains(balance.ReadStdout(), "100000")
	})
	assert.NoError(daemon.Error)
	assert.Equal(daemon.Code, 0)
}
