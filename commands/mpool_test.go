package commands

import (
	"fmt"
	"strings"
	"testing"

	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/types"
)

func TestMpool(t *testing.T) {
	assert := assert.New(t)

	d := withDaemon(func() {
		_ = runSuccess(t, fmt.Sprintf("go-filecoin send-message --from %s --value 10 %s", types.Address("filecoin"), types.Address("investor1")))
		out := runSuccess(t, "go-filecoin mpool")
		c := strings.Trim(out.ReadStdout(), "\n")
		ci, err := cid.Decode(c)
		assert.NoError(err)
		assert.NotNil(ci)
	})
	assert.NoError(d.Error)
	assert.Equal(d.Code, 0)
}
