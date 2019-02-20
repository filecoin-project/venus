package fast

import (
	"context"
	"io/ioutil"
	"math/big"
	"os"
	"testing"

	iptb "github.com/ipfs/iptb/testbed"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"

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

func TestEnvironmentMemoryGenesis(t *testing.T) {
	t.Run("SetupTeardown", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)
		ctx := context.Background()

		testDir, err := ioutil.TempDir(".", "environmentTest")
		require.NoError(err)
		defer os.RemoveAll(testDir)

		env, err := NewEnvironmentMemoryGenesis(big.NewInt(100000), testDir)
		localenv := env.(*EnvironmentMemoryGenesis)
		assert.NoError(err)
		assert.NotNil(env)
		assert.Equal(testDir, localenv.location)

		// did we create the dir correctly?
		_, err = os.Stat(localenv.location)
		assert.NoError(err)

		// did we teardown correctly?
		assert.NoError(env.Teardown(ctx))
		_, existsErr := os.Stat(localenv.location)
		assert.True(os.IsNotExist(existsErr))
	})

	t.Run("ProcessCreateAndTeardown", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)
		ctx := context.Background()

		testDir, err := ioutil.TempDir(".", "environmentTest")
		require.NoError(err)
		defer os.RemoveAll(testDir)

		env, err := NewEnvironmentMemoryGenesis(big.NewInt(100000), testDir)
		require.NoError(err)

		p, err := env.NewProcess(ctx, mockplugin.PluginName, nil, EnvironmentOpts{})
		assert.NoError(err)
		assert.NotNil(p)
		assert.Equal(1, len(env.Processes()))

		// did we create the process dir correctly?
		_, err = os.Stat(p.core.Dir())
		assert.NoError(err)

		assert.NoError(env.TeardownProcess(ctx, p))
		assert.Equal(0, len(env.Processes()))

		// did we teardown the process correctly?
		_, existsErr := os.Stat(p.core.Dir())
		assert.True(os.IsNotExist(existsErr))
	})
}
