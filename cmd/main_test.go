package cmd

import (
	"context"
	"testing"

	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/stretchr/testify/assert"
)

func TestRequiresDaemon(t *testing.T) {
	tf.UnitTest(t)

	reqWithDaemon, err := cmds.NewRequest(context.Background(), []string{"chain", "head"}, nil, []string{}, nil, RootCmdDaemon)
	assert.NoError(t, err)
	assert.True(t, requiresDaemon(reqWithDaemon))

	reqWithoutDaemon, err := cmds.NewRequest(context.Background(), []string{"daemon"}, nil, []string{}, nil, RootCmd)
	assert.NoError(t, err)
	assert.False(t, requiresDaemon(reqWithoutDaemon))

	reqSubcmdDaemon, err := cmds.NewRequest(context.Background(), []string{"version"}, nil, []string{}, nil, RootCmd)
	assert.NoError(t, err)
	assert.False(t, requiresDaemon(reqSubcmdDaemon))
}
