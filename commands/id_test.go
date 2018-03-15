package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestId(t *testing.T) {
	assert := assert.New(t)

	d := NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	id := d.RunSuccess("id")

	idContent := id.ReadStdout()
	assert.Containsf(idContent, "/ip4/127.0.0.1/tcp/6000/ipfs/", "default addr")
	assert.Contains(idContent, "ID")

}

func TestIdFormat(t *testing.T) {
	assert := assert.New(t)

	d := NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	idContent := d.RunSuccess("id",
		"--format=\"<id>\\t<aver>\\t<pver>\\t<pubkey>\\n<addrs>\"",
	).ReadStdout()

	assert.Contains(idContent, "\t")
	assert.Contains(idContent, "\n")
	assert.Containsf(idContent, "/ip4/127.0.0.1/tcp/6000/ipfs/", "default addr")
	assert.NotContains(idContent, "ID")
}
