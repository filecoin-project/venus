package commands

import (
	"strings"
	"sync"
	"testing"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/stretchr/testify/assert"
)

func TestMessageSend(t *testing.T) {
	d := NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	t.Log("[failure] invalid target")
	d.RunFail(
		"invalid checksum",
		"message", "send",
		"--from", core.NetworkAddress.String(),
		"--value=10", "xyz",
	)

	t.Log("[failure] no from")
	d.RunFail("missing from", "message", "send", core.TestAddress.String())

	t.Log("[success] with from")
	d.RunSuccess("message", "send",
		"--from", core.NetworkAddress.String(), core.TestAddress.String(),
	)

	t.Log("[success] with from and value")
	d.RunSuccess("message", "send",
		"--from", core.NetworkAddress.String(),
		"--value=10", core.TestAddress.String(),
	)
}

func TestMessageWait(t *testing.T) {
	d := NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	t.Run("[success] transfer only", func(t *testing.T) {
		assert := assert.New(t)

		msg := d.RunSuccess(
			"message", "send",
			"--from", core.NetworkAddress.String(),
			"--value=10",
			core.TestAddress.String(),
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
