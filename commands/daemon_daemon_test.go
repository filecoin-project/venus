package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDaemonStartupMessage(t *testing.T) {
	assert := assert.New(t)
	daemon := NewDaemon(t).Start()
	daemon.ShutdownSuccess()

	out := daemon.ReadStdout()
	assert.Regexp("^My peer ID is [a-zA-Z0-9]*", out)
	assert.Regexp("\\nSwarm listening on.*", out)
}

func TestDaemonApiFile(t *testing.T) {
	assert := assert.New(t)
	daemon := NewDaemon(t).Start()

	apiPath := filepath.Join(daemon.repoDir, "api")
	assert.FileExists(apiPath)

	daemon.ShutdownEasy()

	_, err := os.Lstat(apiPath)
	assert.Error(err, "Expect api file to be deleted on shutdown")
	assert.True(os.IsNotExist(err))
}
