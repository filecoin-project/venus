package commands

import (
	"testing"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/stretchr/testify/assert"
)

func TestBitswapStats(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	d := th.NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	stats := d.RunSuccess("bitswap", "stats").ReadStdoutTrimNewlines()

	assert.Equal(`{"ProvideBufLen":0,"Wantlist":[],"Peers":[],"BlocksReceived":0,"DataReceived":0,"BlocksSent":0,"DataSent":0,"DupBlksReceived":0,"DupDataReceived":0,"MessagesReceived":0}`, stats)
}
