package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestId(t *testing.T) {
	assert := assert.New(t)

	runWithDaemon("go-filecoin id", func(id *Output) {
		assert.NoError(id.Error)
		assert.Equal(id.Code, 0)
		assert.Empty(id.ReadStderr())

		idContent := id.ReadStdout()
		assert.Containsf(idContent, "/ip4/127.0.0.1/tcp/6000/ipfs/", "default addr")
		assert.Contains(idContent, "ID")
	})
}

func TestIdFormat(t *testing.T) {
	assert := assert.New(t)

	runWithDaemon("go-filecoin id --format=\"<id>\\t<aver>\\t<pver>\\t<pubkey>\\n<addrs>\"", func(id *Output) {
		assert.NoError(id.Error)
		assert.Equal(id.Code, 0)
		assert.Empty(id.ReadStderr())

		idContent := id.ReadStdout()
		assert.Contains(idContent, "\t")
		assert.Contains(idContent, "\n")
		assert.Containsf(idContent, "/ip4/127.0.0.1/tcp/6000/ipfs/", "default addr")
		assert.NotContains(idContent, "ID")
	})
}
