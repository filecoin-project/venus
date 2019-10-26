package commands_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/fixtures"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestMpoolLs(t *testing.T) {
	tf.IntegrationTest(t)

	sendMessage := func(d *th.TestDaemon, from string, to string) *th.CmdOutput {
		return d.RunSuccess("message", "send",
			"--from", from,
			"--gas-price", "1", "--gas-limit", "300",
			"--value=10", to,
		)
	}

	t.Run("return all messages", func(t *testing.T) {

		d := th.NewDaemon(t, th.KeyFile(fixtures.KeyFilePaths()[0])).Start()
		defer d.ShutdownSuccess()

		sendMessage(d, fixtures.TestAddresses[0], fixtures.TestAddresses[2])
		sendMessage(d, fixtures.TestAddresses[0], fixtures.TestAddresses[2])

		out := d.RunSuccess("mpool", "ls")

		cids := strings.Split(strings.Trim(out.ReadStdout(), "\n"), "\n")
		assert.Equal(t, 2, len(cids))

		for _, c := range cids {
			ci, err := cid.Decode(c)
			assert.NoError(t, err)
			assert.True(t, ci.Defined())
		}

		// Should return immediately with --wait-for-count equal to message count
		out = d.RunSuccess("mpool", "ls", "--wait-for-count=2")
		cids = strings.Split(strings.Trim(out.ReadStdout(), "\n"), "\n")
		assert.Equal(t, 2, len(cids))
	})

	t.Run("wait for enough messages", func(t *testing.T) {

		d := th.NewDaemon(t, th.KeyFile(fixtures.KeyFilePaths()[0])).Start()
		defer d.ShutdownSuccess()

		wg := sync.WaitGroup{}
		wg.Add(1)

		complete := false
		go func() {
			out := d.RunSuccess("mpool", "ls", "--wait-for-count=3")
			complete = true
			c := strings.Split(strings.Trim(out.ReadStdout(), "\n"), "\n")
			assert.Equal(t, 3, len(c))
			wg.Done()
		}()

		sendMessage(d, fixtures.TestAddresses[0], fixtures.TestAddresses[1])
		assert.False(t, complete)
		sendMessage(d, fixtures.TestAddresses[0], fixtures.TestAddresses[1])
		assert.False(t, complete)
		sendMessage(d, fixtures.TestAddresses[0], fixtures.TestAddresses[1])

		wg.Wait()

		assert.True(t, complete)
	})
}

func TestMpoolShow(t *testing.T) {
	tf.IntegrationTest(t)

	t.Run("shows message", func(t *testing.T) {

		d := th.NewDaemon(t, th.KeyFile(fixtures.KeyFilePaths()[0])).Start()
		defer d.ShutdownSuccess()

		msgCid := d.RunSuccess("message", "send",
			"--from", fixtures.TestAddresses[0],
			"--gas-price", "1", "--gas-limit", "300",
			"--value=10", fixtures.TestAddresses[2],
		).ReadStdoutTrimNewlines()

		out := d.RunSuccess("mpool", "show", msgCid).ReadStdoutTrimNewlines()

		assert.Contains(t, out, "From:      "+fixtures.TestAddresses[0])
		assert.Contains(t, out, "To:        "+fixtures.TestAddresses[2])
		assert.Contains(t, out, "Value:     10")
	})

	t.Run("fails missing message", func(t *testing.T) {

		d := th.NewDaemon(t, th.KeyFile(fixtures.KeyFilePaths()[0])).Start()
		defer d.ShutdownSuccess()

		const c = "QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw"

		out := d.RunFail("not found", "mpool", "show", c).ReadStderr()
		assert.Contains(t, out, c)
	})
}

func TestMpoolRm(t *testing.T) {
	tf.IntegrationTest(t)

	t.Run("remove a message", func(t *testing.T) {

		d := th.NewDaemon(t, th.KeyFile(fixtures.KeyFilePaths()[0])).Start()
		defer d.ShutdownSuccess()

		msgCid := d.RunSuccess("message", "send",
			"--from", fixtures.TestAddresses[0],
			"--gas-price", "1", "--gas-limit", "300",
			"--value=10", fixtures.TestAddresses[2],
		).ReadStdoutTrimNewlines()

		d.RunSuccess("mpool", "rm", msgCid)

		out := d.RunSuccess("mpool", "ls").ReadStdoutTrimNewlines()
		assert.Equal(t, "", out)
	})
}
