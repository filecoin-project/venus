package commands_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/fixtures"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestOutbox(t *testing.T) {
	tf.IntegrationTest(t)

	sendMessage := func(d *th.TestDaemon, from string, to string) *th.CmdOutput {
		return d.RunSuccess("message", "send",
			"--from", from,
			"--gas-price", "1", "--gas-limit", "300",
			"--value=10", to,
		)
	}

	t.Run("list queued messages", func(t *testing.T) {

		d := th.NewDaemon(t, th.KeyFile(fixtures.KeyFilePaths()[0]), th.KeyFile(fixtures.KeyFilePaths()[1])).Start()
		defer d.ShutdownSuccess()

		c1 := sendMessage(d, fixtures.TestAddresses[0], fixtures.TestAddresses[2]).ReadStdoutTrimNewlines()
		c2 := sendMessage(d, fixtures.TestAddresses[0], fixtures.TestAddresses[2]).ReadStdoutTrimNewlines()
		c3 := sendMessage(d, fixtures.TestAddresses[1], fixtures.TestAddresses[2]).ReadStdoutTrimNewlines()

		out := d.RunSuccess("outbox", "ls").ReadStdout()
		assert.Contains(t, out, fixtures.TestAddresses[0])
		assert.Contains(t, out, fixtures.TestAddresses[1])
		assert.Contains(t, out, c1)
		assert.Contains(t, out, c2)
		assert.Contains(t, out, c3)

		// With address filter
		out = d.RunSuccess("outbox", "ls", fixtures.TestAddresses[1]).ReadStdout()
		assert.NotContains(t, out, fixtures.TestAddresses[0])
		assert.Contains(t, out, fixtures.TestAddresses[1])
		assert.NotContains(t, out, c1)
		assert.NotContains(t, out, c2)
		assert.Contains(t, out, c3)
	})

	t.Run("clear queue", func(t *testing.T) {

		d := th.NewDaemon(t, th.KeyFile(fixtures.KeyFilePaths()[0]), th.KeyFile(fixtures.KeyFilePaths()[1])).Start()
		defer d.ShutdownSuccess()

		c1 := sendMessage(d, fixtures.TestAddresses[0], fixtures.TestAddresses[2]).ReadStdoutTrimNewlines()
		c2 := sendMessage(d, fixtures.TestAddresses[0], fixtures.TestAddresses[2]).ReadStdoutTrimNewlines()
		c3 := sendMessage(d, fixtures.TestAddresses[1], fixtures.TestAddresses[2]).ReadStdoutTrimNewlines()

		// With address filter
		d.RunSuccess("outbox", "clear", fixtures.TestAddresses[1])
		out := d.RunSuccess("outbox", "ls").ReadStdout()
		assert.Contains(t, out, fixtures.TestAddresses[0])
		assert.NotContains(t, out, fixtures.TestAddresses[1]) // Cleared
		assert.Contains(t, out, c1)
		assert.Contains(t, out, c2)
		assert.NotContains(t, out, c3) // cleared

		// Repopulate
		sendMessage(d, fixtures.TestAddresses[1], fixtures.TestAddresses[2]).ReadStdoutTrimNewlines()

		// #nofilter
		d.RunSuccess("outbox", "clear")
		out = d.RunSuccess("outbox", "ls").ReadStdoutTrimNewlines()
		assert.Empty(t, out)

		// Clearing empty queue
		d.RunSuccess("outbox", "clear")
		out = d.RunSuccess("outbox", "ls").ReadStdoutTrimNewlines()
		assert.Empty(t, out)
	})
}
