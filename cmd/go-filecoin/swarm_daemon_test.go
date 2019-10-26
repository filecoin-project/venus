package commands_test

import (
	"testing"

	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestSwarmConnectPeersValid(t *testing.T) {
	tf.IntegrationTest(t)

	d1 := th.NewDaemon(t).Start()
	defer d1.ShutdownSuccess()

	d2 := th.NewDaemon(t).Start()
	defer d2.ShutdownSuccess()

	d1.ConnectSuccess(d2)
}

func TestSwarmConnectPeersInvalid(t *testing.T) {
	tf.IntegrationTest(t)

	d1 := th.NewDaemon(t).Start()
	defer d1.ShutdownSuccess()

	d1.RunFail("failed to parse ip4 addr",
		"swarm connect /ip4/hello",
	)
}
