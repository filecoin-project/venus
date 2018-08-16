package commands

import (
	"strings"
	"sync"
	"testing"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/stretchr/testify/assert"
)

func TestMessageSend(t *testing.T) {
	t.Parallel()

	d := th.NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	d.RunSuccess("mining", "once")

	t.Log("[failure] invalid target")
	d.RunFail(
		"invalid checksum",
		"message", "send",
		"--from", th.TestAddress1,
		"--value=10", "xyz",
	)

	t.Log("[success] default from")
	d.RunSuccess("message", "send", th.TestAddress1)

	t.Log("[success] with from")
	d.RunSuccess("message", "send",
		"--from", th.TestAddress1, th.TestAddress2,
	)

	t.Log("[success] with from and value")
	d.RunSuccess("message", "send",
		"--from", th.TestAddress1,
		"--value=10", th.TestAddress2,
	)
}

func TestMessageWait(t *testing.T) {
	t.Parallel()

	d := th.NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	t.Run("[success] transfer only", func(t *testing.T) {
		assert := assert.New(t)

		msg := d.RunSuccess(
			"message", "send",
			"--from", th.TestAddress1,
			"--value=10",
			th.TestAddress2,
		)

		msgcid := strings.Trim(msg.ReadStdout(), "\n")

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			wait := d.RunSuccess(
				"message", "wait",
				"--message=false",
				"--receipt=false",
				"--return",
				msgcid,
			)
			// nothing should be printed, as there is no return value
			assert.Equal("", wait.ReadStdout())
			wg.Done()
		}()

		d.RunSuccess("mining once")

		wg.Wait()
	})
}
