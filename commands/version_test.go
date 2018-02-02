package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/flags"
	"github.com/filecoin-project/go-filecoin/testhelpers"
)

func TestVersion(t *testing.T) {
	assert := assert.New(t)
	flags.Commit = "12345"

	env := Env{}
	out, err := testhelpers.RunCommand(versionCmd, []string{"version"}, &env)
	assert.NoError(err)

	assert.Contains(out, "commit: 12345")
}
