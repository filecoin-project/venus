package commands_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

func TestProtocol(t *testing.T) {
	tf.IntegrationTest(t)

	d := th.NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	protocol := d.RunSuccess("protocol")

	protocolContent := protocol.ReadStdout()
	assert.Contains(t, protocolContent, "Auto-Seal Interval: 120 seconds")
}
