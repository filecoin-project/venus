package commands_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

func TestPing2Nodes(t *testing.T) {
	tf.IntegrationTest(t)

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
	assert.Contains(t, ping1.ReadStdout(), "Pong received")

	t.Log("[success] 2 -> 1")
	assert.Contains(t, ping2.ReadStdout(), "Pong received")
}
