package commands

import (
	"testing"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
)

func TestSwarmConnectPeersValid(t *testing.T) {
	d1 := th.NewDaemon(t, th.SwarmAddr("/ip4/0.0.0.0/tcp/6000")).Start()
	defer d1.ShutdownSuccess()

	d2 := th.NewDaemon(t, th.SwarmAddr("/ip4/0.0.0.0/tcp/6001")).Start()
	defer d2.ShutdownSuccess()

	d1.ConnectSuccess(d2)
}

func TestSwarmConnectPeersInvalid(t *testing.T) {
	d1 := th.NewDaemon(t, th.SwarmAddr("/ip4/0.0.0.0/tcp/6000")).Start()
	defer d1.ShutdownSuccess()

	d1.RunFail("failed to parse ip4 addr",
		"swarm connect /ip4/hello",
	)
}
