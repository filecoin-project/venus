package commands_test

import (
	"testing"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
)

func TestBitswapStats(t *testing.T) {
	tf.IntegrationTest(t)

	d := th.NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	stats := d.RunSuccess("bitswap", "stats").ReadStdoutTrimNewlines()

	assert.Equal(t, `{"ProvideBufLen":0,"Wantlist":[],"Peers":[],"BlocksReceived":0,"DataReceived":0,"BlocksSent":0,"DataSent":0,"DupBlksReceived":0,"DupDataReceived":0,"MessagesReceived":0}`, stats)
}
