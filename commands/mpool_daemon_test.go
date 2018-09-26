package commands

import (
	"strings"
	"testing"

	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"

	"github.com/stretchr/testify/assert"

	"sync"

	"github.com/filecoin-project/go-filecoin/fixtures"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
)

func TestMpool(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	t.Run("return all messages", func(t *testing.T) {
		t.Parallel()
		d := th.NewDaemon(t, th.KeyFile(fixtures.KeyFilePaths()[0])).Start()
		defer d.ShutdownSuccess()

		d.RunSuccess("message", "send",
			"--from", fixtures.TestAddresses[0],
			"--value=10", fixtures.TestAddresses[2],
		)

		out := d.RunSuccess("mpool")
		c := strings.Trim(out.ReadStdout(), "\n")
		ci, err := cid.Decode(c)
		assert.NoError(err)
		assert.NotNil(ci)
	})

	t.Run("wait for enough messages", func(t *testing.T) {
		t.Parallel()
		d := th.NewDaemon(t, th.KeyFile(fixtures.KeyFilePaths()[0])).Start()
		defer d.ShutdownSuccess()

		wg := sync.WaitGroup{}
		wg.Add(1)

		complete := false
		go func() {
			out := d.RunSuccess("mpool", "--wait-for-count=3")
			complete = true
			c := strings.Split(strings.Trim(out.ReadStdout(), "\n"), "\n")
			assert.Equal(3, len(c))
			wg.Done()
		}()

		d.RunSuccess("message", "send",
			"--from", fixtures.TestAddresses[0],
			"--value=10", fixtures.TestAddresses[1],
		)

		assert.False(complete)

		d.RunSuccess("message", "send",
			"--from", fixtures.TestAddresses[0],
			"--value=10", fixtures.TestAddresses[1],
		)

		assert.False(complete)

		d.RunSuccess("message", "send",
			"--from", fixtures.TestAddresses[0],
			"--value=10", fixtures.TestAddresses[1],
		)

		wg.Wait()

		assert.True(complete)
	})
}
