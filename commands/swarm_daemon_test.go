package commands_test

import (
	"testing"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"

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

func TestSwarmFindPeer(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	d1 := th.NewDaemon(t).Start()
	defer d1.ShutdownSuccess()

	d2 := th.NewDaemon(t).Start()
	defer d2.ShutdownSuccess()

	d1.ConnectSuccess(d2)

	d2Id := d2.GetID()

	findpeerOutput := d1.RunSuccess("swarm", "findpeer", d2Id).ReadStdoutTrimNewlines()

	d2Addr := d2.GetAddresses()[0]

	assert.Contains(d2Addr, findpeerOutput)
}
