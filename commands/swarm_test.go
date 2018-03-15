package commands

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSwarmConnectPeers(t *testing.T) {
	assert := assert.New(t)

	d1 := NewDaemon(t, SwarmAddr("/ip4/127.0.0.1/tcp/6000")).Start()
	defer d1.ShutdownSuccess()

	d2 := NewDaemon(t, SwarmAddr("/ip4/127.0.0.1/tcp/6001")).Start()
	defer d2.ShutdownSuccess()

	t.Log("[failure] invalid peer")
	d1.RunFail("invalid peer address", "swarm connect", "/ip4/hello")

	d1.RunSuccess("swarm connect",
		fmt.Sprintf("/ip4/127.0.0.1/tcp/6001/ipfs/%s", d2.GetID()))
	peers1 := d1.RunSuccess("swarm peers")
	peers2 := d2.RunSuccess("swarm peers")

	t.Log("[success] 1 -> 2")
	assert.Contains(peers1.ReadStdout(), d2.GetID())

	t.Log("[success] 2 -> 1")
	assert.Contains(peers2.ReadStdout(), d1.GetID())

}
