package commands_test

import (
	"testing"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
)

func TestPing2Nodes(t *testing.T) {
	assert := assert.New(t)

	d1 := th.NewDaemon(t, th.SwarmAddr("/ip4/127.0.0.1/tcp/6000")).Start()
	defer d1.ShutdownSuccess()
	d2 := th.NewDaemon(t, th.SwarmAddr("/ip4/127.0.0.1/tcp/6001")).Start()
	defer d2.ShutdownSuccess()

	t.Log("[failure] not connected")
	d1.RunFail("failed to find any peer in table",
		"ping", "--count=2", d2.GetID(),
	)

	d1.ConnectSuccess(d2)
	ping1 := d1.RunSuccess("ping", "--count=2", d2.GetID())
	ping2 := d2.RunSuccess("ping", "--count=2", d1.GetID())

	t.Log("[success] 1 -> 2")
	assert.Contains(ping1.ReadStdout(), "Pong received")

	t.Log("[success] 2 -> 1")
	assert.Contains(ping2.ReadStdout(), "Pong received")
}
