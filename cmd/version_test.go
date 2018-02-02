package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/flags"
	"github.com/filecoin-project/go-filecoin/testhelpers"
)

func TestCmd_Version(t *testing.T) {
	assert := assert.New(t)
	flags.Commit = "12345"

	out, err := testhelpers.RunCommand(versionCmd, []string{"version"})
	assert.NoError(err)

	assert.Contains(out, "commit: 12345")
}
