package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/flags"
)

func TestVersion(t *testing.T) {
	assert := assert.New(t)
	flags.Commit = "12345"

	runWithDaemon("go-filecoin version", func(out *Output) {
		assert.NoError(out.Error)
		assert.Equal(out.Code, 0)
		assert.Equal(out.ReadStderr(), "")
		assert.Contains(out.ReadStdout(), "commit: 12345")
	})
}
