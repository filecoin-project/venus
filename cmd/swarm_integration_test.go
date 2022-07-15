package cmd_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/venus/app/node/test"
	th "github.com/filecoin-project/venus/pkg/testhelpers"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"

	"github.com/stretchr/testify/assert"
)

func TestSwarmConnectPeersValid(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()
	builder := test.NewNodeBuilder(t)

	n1 := builder.BuildAndStart(ctx)
	defer n1.Stop(ctx)
	n2 := builder.BuildAndStart(ctx)
	defer n2.Stop(ctx)

	test.ConnectNodes(t, n1, n2)
}

func TestSwarmConnectPeersInvalid(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()
	builder := test.NewNodeBuilder(t)

	_, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	cmdClient.RunFail(ctx, "failed to parse ip4 addr",
		"swarm", "connect", "/ip4/hello",
	)
}

func TestId(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()

	builder := test.NewNodeBuilder(t)
	_, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	id := cmdClient.RunSuccess(ctx, "swarm", "id")

	idContent := id.ReadStdout()
	assert.Containsf(t, idContent, "/ip4/127.0.0.1/tcp/", "default addr")
	assert.Contains(t, idContent, "ID")
}

func TestPersistId(t *testing.T) {
	tf.IntegrationTest(t)

	// we need to control this
	dir := t.TempDir()

	// Start a demon in dir
	d1 := th.NewDaemon(t, th.ContainerDir(dir)).Start()

	// get the id and kill it
	id1 := d1.GetID()
	d1.Stop()

	// restart the daemon
	d2 := th.NewDaemon(t, th.ContainerDir(dir)).Start()

	// get the id and compare to previous
	id2 := d2.GetID()
	d2.ShutdownSuccess()
	t.Logf("d1: %s", d1.ReadStdout())
	t.Logf("d2: %s", d2.ReadStdout())
	assert.Equal(t, id1, id2)
}

func TestDhtFindPeer(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()

	builder1 := test.NewNodeBuilder(t)
	n1 := builder1.BuildAndStart(ctx)
	defer n1.Stop(ctx)
	cmdClient, done := test.RunNodeAPI(ctx, n1, t)
	defer done()

	builder2 := test.NewNodeBuilder(t)
	n2 := builder2.BuildAndStart(ctx)
	defer n2.Stop(ctx)

	test.ConnectNodes(t, n1, n2)

	pi, err := n2.Network().API().NetAddrsListen(ctx)
	assert.Nil(t, err)
	findpeerOutput := cmdClient.RunSuccess(ctx, "swarm", "findpeer", pi.ID.Pretty()).ReadStdoutTrimNewlines()

	assert.Contains(t, findpeerOutput, pi.Addrs[0].String())
}

func TestStatsBandwidth(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()
	builder := test.NewNodeBuilder(t)

	_, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	stats := cmdClient.RunSuccess(ctx, "swarm", "bandwidth").ReadStdoutTrimNewlines()

	assert.Equal(t, "Segment  TotalIn  TotalOut  RateIn  RateOut\nTotal    0 B      0 B       0 B/s   0 B/s", stats)
}
