package commands

import (
	"context"
	"testing"

	"github.com/ipfs/go-ipfs-cmds"
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
