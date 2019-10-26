package commands

import (
	"context"
	"testing"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/ipfs/go-ipfs-cmds"
	"github.com/stretchr/testify/assert"
)

func TestRequiresDaemon(t *testing.T) {
	tf.UnitTest(t)

	reqWithDaemon, err := cmds.NewRequest(context.Background(), []string{"chain", "head"}, nil, []string{}, nil, rootCmdDaemon)
	assert.NoError(t, err)
	assert.True(t, requiresDaemon(reqWithDaemon))

	reqWithoutDaemon, err := cmds.NewRequest(context.Background(), []string{"daemon"}, nil, []string{}, nil, rootCmd)
	assert.NoError(t, err)
	assert.False(t, requiresDaemon(reqWithoutDaemon))

	reqSubcmdDaemon, err := cmds.NewRequest(context.Background(), []string{"leb128", "decode"}, nil, []string{"A=="}, nil, rootCmd)
	assert.NoError(t, err)
	assert.False(t, requiresDaemon(reqSubcmdDaemon))
}
