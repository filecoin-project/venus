package commands

import (
	"testing"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
)

func TestStatsBandwidth(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	d := th.NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	stats := d.RunSuccess("stats", "bandwidth").ReadStdoutTrimNewlines()

	assert.Equal("{\"TotalIn\":0,\"TotalOut\":0,\"RateIn\":0,\"RateOut\":0}", stats)
}
