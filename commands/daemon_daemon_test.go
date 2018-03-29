package commands

import (
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
