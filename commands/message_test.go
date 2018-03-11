package commands

import (
	"fmt"
	"testing"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
)

func TestMessageSend(t *testing.T) {
	assert := assert.New(t)

	d := withDaemon(func() {
		t.Log("[failure] invalid target")
		fail := run(fmt.Sprintf("go-filecoin message send --from %s --value 10 xyz", types.Address("filecoin")))
		assert.Equal(fail.Code, 1)
		assert.NoError(fail.Error)
		assert.Contains(fail.ReadStderr(), "addresses must start with 0x")

		t.Log("[failure] no from and no addresses")
		fail = run(fmt.Sprintf("go-filecoin message send %s", types.Address("investor1")))
		assert.Equal(fail.Code, 1)
		assert.NoError(fail.Error)
		assert.Contains(fail.ReadStderr(), "no default address")

		t.Log("[success] no from")
		_ = runSuccess(t, "go-filecoin wallet addrs new")
		_ = runSuccess(t, fmt.Sprintf("go-filecoin message send %s", types.Address("investor1")))

		t.Log("[success] with from")
		_ = runSuccess(t, fmt.Sprintf("go-filecoin message send --from %s %s", types.Address("filecoin"), types.Address("investor1")))

		t.Log("[success] with from and value")
		_ = runSuccess(t, fmt.Sprintf("go-filecoin message send --from %s --value 10 %s", types.Address("filecoin"), types.Address("investor1")))
	})
	assert.NoError(d.Error)
	assert.Equal(d.Code, 0)
}
