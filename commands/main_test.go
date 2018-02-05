package commands

import (
	"context"
	"net"
	"testing"

	cmds "gx/ipfs/Qmc5paX4ECBARnAKkcAmUYHBGor228Tkfxeya3Nu2KRL46/go-ipfs-cmds"

	"github.com/stretchr/testify/assert"
)

func TestRequiresDaemon(t *testing.T) {
	assert := assert.New(t)

	reqWithDaemon, err := cmds.NewRequest(context.Background(), []string{}, nil, []string{"chain"}, nil, rootSubcmdsDaemon["chain"])
	assert.NoError(err)

	reqWithoutDaemon, err := cmds.NewRequest(context.Background(), []string{}, nil, []string{"daemon"}, nil, rootSubcmdsNoDaemon["daemon"])
	assert.NoError(err)

	assert.True(requiresDaemon(reqWithDaemon))
	assert.False(requiresDaemon(reqWithoutDaemon))
}

func TestDaemonRunning(t *testing.T) {
	assert := assert.New(t)

	// No daemon running
	isRunning, err := daemonRunning(":3456")
	assert.NoError(err)
	assert.False(isRunning)

	// something is running on this port

	ln, err := net.Listen("tcp", ":3456")
	assert.NoError(err)
	defer ln.Close()

	isRunning, err = daemonRunning(":3456")
	assert.NoError(err)
	assert.True(isRunning)
}
