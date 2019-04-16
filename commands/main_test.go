package commands

import (
	"context"
	"testing"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/ipfs/go-ipfs-cmds"
	"github.com/stretchr/testify/assert"
)

func TestRequiresDaemon(t *testing.T) {
	tf.UnitTest(t)

	assert := assert.New(t)

	reqWithDaemon, err := cmds.NewRequest(context.Background(), []string{}, nil, []string{"chain"}, nil, chainCmd)
	assert.NoError(err)

	reqWithoutDaemon, err := cmds.NewRequest(context.Background(), []string{}, nil, []string{"daemon"}, nil, daemonCmd)
	assert.NoError(err)

	assert.True(requiresDaemon(reqWithDaemon))
	assert.False(requiresDaemon(reqWithoutDaemon))
}
