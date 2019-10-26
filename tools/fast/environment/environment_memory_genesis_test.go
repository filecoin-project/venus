package environment

import (
	"context"
	"io/ioutil"
	"math/big"
	"os"
	"testing"

	iptb "github.com/ipfs/iptb/testbed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	mockplugin "github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin/mock"
)

// must register all filecoin iptb plugins
func init() {
	_, err := iptb.RegisterPlugin(iptb.IptbPlugin{
		From:       "<builtin>",
		NewNode:    mockplugin.NewNode,
		PluginName: mockplugin.PluginName,
		BuiltIn:    true,
	}, false)

	if err != nil {
		panic(err)
	}
}

func TestMemoryGenesis(t *testing.T) {
	tf.UnitTest(t)

	t.Run("SetupTeardown", func(t *testing.T) {
		ctx := context.Background()

		testDir, err := ioutil.TempDir(".", "environmentTest")
		require.NoError(t, err)
		defer func() {
			require.NoError(t, os.RemoveAll(testDir))
		}()

		env, err := NewMemoryGenesis(big.NewInt(100000), testDir, types.TestProofsMode)
		localenv := env.(*MemoryGenesis)
		assert.NoError(t, err)
		assert.NotNil(t, env)
		assert.Equal(t, testDir, localenv.location)

		// did we create the dir correctly?
		_, err = os.Stat(localenv.location)
		assert.NoError(t, err)

		// did we teardown correctly?
		assert.NoError(t, env.Teardown(ctx))
		assert.Equal(t, 0, len(env.Processes()))
		_, existsErr := os.Stat(localenv.location)
		assert.True(t, os.IsNotExist(existsErr))
	})

	t.Run("ProcessCreateAndTeardown", func(t *testing.T) {
		ctx := context.Background()

		testDir, err := ioutil.TempDir(".", "environmentTest")
		require.NoError(t, err)
		defer func() {
			require.NoError(t, os.RemoveAll(testDir))
		}()

		env, err := NewMemoryGenesis(big.NewInt(100000), testDir, types.TestProofsMode)
		require.NoError(t, err)

		p, err := env.NewProcess(ctx, mockplugin.PluginName, nil, fast.FilecoinOpts{})
		assert.NoError(t, err)
		assert.NotNil(t, p)
		assert.Equal(t, 1, len(env.Processes()))

		// did we create the process dir correctly?
		_, err = os.Stat(p.Dir())
		assert.NoError(t, err)

		assert.NoError(t, env.TeardownProcess(ctx, p))
		assert.Equal(t, 0, len(env.Processes()))

		// did we teardown the process correctly?
		_, existsErr := os.Stat(p.Dir())
		assert.True(t, os.IsNotExist(existsErr))
	})
}
