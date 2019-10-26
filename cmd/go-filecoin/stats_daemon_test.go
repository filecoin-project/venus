package commands_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestStatsBandwidth(t *testing.T) {
	tf.IntegrationTest(t)

	d := th.NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	stats := d.RunSuccess("stats", "bandwidth").ReadStdoutTrimNewlines()

	assert.Equal(t, "{\"TotalIn\":0,\"TotalOut\":0,\"RateIn\":0,\"RateOut\":0}", stats)
}
