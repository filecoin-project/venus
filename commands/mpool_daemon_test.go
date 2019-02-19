package commands

import (
	"strings"
	"sync"
	"testing"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/fixtures"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
)

func TestMpoolLs(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	t.Run("return all messages", func(t *testing.T) {
		t.Parallel()
		d := th.NewDaemon(t, th.KeyFile(fixtures.KeyFilePaths()[0])).Start()
		defer d.ShutdownSuccess()

		sendMessage(d, fixtures.TestAddresses[0], fixtures.TestAddresses[2])
		sendMessage(d, fixtures.TestAddresses[0], fixtures.TestAddresses[2])

		out := d.RunSuccess("mpool", "ls")

		cids := strings.Split(strings.Trim(out.ReadStdout(), "\n"), "\n")
		assert.Equal(2, len(cids))

		for _, c := range cids {
			ci, err := cid.Decode(c)
			assert.NoError(err)
			assert.True(ci.Defined())
		}

		// Should return immediately with --wait-for-count equal to message count
		out = d.RunSuccess("mpool", "ls", "--wait-for-count=2")
		cids = strings.Split(strings.Trim(out.ReadStdout(), "\n"), "\n")
		assert.Equal(2, len(cids))
	})

	t.Run("wait for enough messages", func(t *testing.T) {
		t.Parallel()
		d := th.NewDaemon(t, th.KeyFile(fixtures.KeyFilePaths()[0])).Start()
		defer d.ShutdownSuccess()

		wg := sync.WaitGroup{}
		wg.Add(1)

		complete := false
		go func() {
			out := d.RunSuccess("mpool", "ls", "--wait-for-count=3")
			complete = true
			c := strings.Split(strings.Trim(out.ReadStdout(), "\n"), "\n")
			assert.Equal(3, len(c))
			wg.Done()
		}()

		sendMessage(d, fixtures.TestAddresses[0], fixtures.TestAddresses[1])
		assert.False(complete)
		sendMessage(d, fixtures.TestAddresses[0], fixtures.TestAddresses[1])
		assert.False(complete)
		sendMessage(d, fixtures.TestAddresses[0], fixtures.TestAddresses[1])

		wg.Wait()

		assert.True(complete)
	})
}

func TestMpoolRm(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	t.Run("remove a message", func(t *testing.T) {
		t.Parallel()
		d := th.NewDaemon(t, th.KeyFile(fixtures.KeyFilePaths()[0])).Start()
		defer d.ShutdownSuccess()

		msgCid := d.RunSuccess("message", "send",
			"--from", fixtures.TestAddresses[0],
			"--price", "0", "--limit", "300",
			"--value=10", fixtures.TestAddresses[2],
		).ReadStdoutTrimNewlines()

		d.RunSuccess("mpool", "rm", msgCid)

		out := d.RunSuccess("mpool", "ls").ReadStdoutTrimNewlines()
		assert.Equal("", out)
	})
}

func sendMessage(d *th.TestDaemon, from string, to string) *th.Output {
	return d.RunSuccess("message", "send",
		"--from", from,
		"--price", "0", "--limit", "300",
		"--value=10", to,
	)
}
