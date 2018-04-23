package commands

import (
	"context"
	"net"
	"testing"

	cmds "gx/ipfs/QmUf5GFfV2Be3UtSAPKDVkoRd1TwEBTmx9TSSCFGGjNgdQ/go-ipfs-cmds"

	"github.com/stretchr/testify/assert"
)

func TestRequiresDaemon(t *testing.T) {
	assert := assert.New(t)

	reqWithDaemon, err := cmds.NewRequest(context.Background(), []string{}, nil, []string{"chain"}, nil, chainCmd)
	assert.NoError(err)

	reqWithoutDaemon, err := cmds.NewRequest(context.Background(), []string{}, nil, []string{"daemon"}, nil, daemonCmd)
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
