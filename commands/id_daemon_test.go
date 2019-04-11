package commands_test

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

func TestId(t *testing.T) {
	tf.IntegrationTest(t)

	assert := assert.New(t)

	d := th.NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	id := d.RunSuccess("id")

	idContent := id.ReadStdout()
	assert.Containsf(idContent, d.SwarmAddr(), "default addr")
	assert.Contains(idContent, "ID")

}

func TestIdFormat(t *testing.T) {
	tf.IntegrationTest(t)

	assert := assert.New(t)

	d := th.NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	idContent := d.RunSuccess("id",
		"--format=\"<id>\\t<aver>\\t<pver>\\t<pubkey>\\n<addrs>\"",
	).ReadStdout()

	assert.Contains(idContent, "\t")
	assert.Contains(idContent, "\n")
	assert.Containsf(idContent, d.SwarmAddr(), "default addr")
	assert.NotContains(idContent, "ID")
}

func TestPersistId(t *testing.T) {
	tf.IntegrationTest(t)

	assert := assert.New(t)

	// we need to control this
	dir, err := ioutil.TempDir("", "go-fil-test")
	require.NoError(t, err)

	// Start a demon in dir
	d1 := th.NewDaemon(t, th.RepoDir(dir)).Start()

	// get the id and kill it
	id1 := d1.GetID()
	d1.Stop()

	// restart the daemon
	d2 := th.NewDaemon(t, th.ShouldInit(false), th.RepoDir(dir)).Start()

	// get the id and compare to previous
	id2 := d2.GetID()
	d2.ShutdownSuccess()
	t.Logf("d1: %s", d1.ReadStdout())
	t.Logf("d2: %s", d2.ReadStdout())
	assert.Equal(id1, id2)
}
