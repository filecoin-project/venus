package commands

import (
	"strings"
	"testing"

	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/core"
)

func TestMpool(t *testing.T) {
	assert := assert.New(t)

	d := NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	d.RunSuccess("message", "send",
		"--from", core.NetworkAddress.String(),
		"--value=10", core.TestAddress.String(),
	)

	out := d.RunSuccess("mpool")
	c := strings.Trim(out.ReadStdout(), "\n")
	ci, err := cid.Decode(c)
	assert.NoError(err)
	assert.NotNil(ci)
}
