package commands

import (
	"testing"
)

func TestSwarmConnectPeers(t *testing.T) {

	d1 := NewDaemon(t, SwarmAddr("/ip4/127.0.0.1/tcp/6000")).Start()
	defer d1.ShutdownSuccess()

	d2 := NewDaemon(t, SwarmAddr("/ip4/127.0.0.1/tcp/6001")).Start()
	defer d2.ShutdownSuccess()

	t.Log("[failure] invalid peer")
	d1.RunFail("invalid peer address", "swarm connect", "/ip4/hello")

	d1.ConnectSuccess(d2)

}
