package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPing2Nodes(t *testing.T) {
	assert := assert.New(t)

	d1 := NewDaemon(t, SwarmAddr("/ip4/127.0.0.1/tcp/6000")).Start()
	defer d1.ShutdownSuccess()
	d2 := NewDaemon(t, SwarmAddr("/ip4/127.0.0.1/tcp/6001")).Start()
	defer d2.ShutdownSuccess()

	t.Log("[failure] not connected")
	d1.RunFail("failed to dial",
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
