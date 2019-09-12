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

	reqWithDaemon, err := cmds.NewRequest(context.Background(), []string{"chain"}, nil, []string{}, nil, rootCmdDaemon)
	assert.NoError(t, err)

	reqWithoutDaemon, err := cmds.NewRequest(context.Background(), []string{"daemon"}, nil, []string{}, nil, rootCmd)
	assert.NoError(t, err)

	assert.True(t, requiresDaemon(reqWithDaemon))
	assert.False(t, requiresDaemon(reqWithoutDaemon))
}
