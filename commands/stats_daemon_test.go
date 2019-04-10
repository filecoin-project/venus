package commands_test

import (
	"testing"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
)

func TestStatsBandwidth(t *testing.T) {
	tf.IntegrationTest(t)

	assert := assert.New(t)

	d := th.NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	stats := d.RunSuccess("stats", "bandwidth").ReadStdoutTrimNewlines()

	assert.Equal("{\"TotalIn\":0,\"TotalOut\":0,\"RateIn\":0,\"RateOut\":0}", stats)
}
