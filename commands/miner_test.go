package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMinerGenBlock(t *testing.T) {
	assert := assert.New(t)

	d := withDaemon(func() {
		t.Log("[failure] no addresses")
		fail := run("go-filecoin miner gen-block")
		assert.Equal(fail.Code, 1)
		assert.NoError(fail.Error)
		assert.Contains(fail.ReadStderr(), "no addresses in wallet to mine")

		t.Log("[success] address in local wallet")
		_ = runSuccess(t, "go-filecoin wallet addrs new")
		_ = runSuccess(t, "go-filecoin miner gen-block")
	})
	assert.NoError(d.Error)
	assert.Equal(d.Code, 0)
}
