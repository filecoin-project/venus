package commands

import (
	"io/ioutil"
	"testing"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

func TestId(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	d := th.NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	id := d.RunSuccess("id")

	idContent := id.ReadStdout()
	assert.Containsf(idContent, d.SwarmAddr(), "default addr")
	assert.Contains(idContent, "ID")

}

func TestIdFormat(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
