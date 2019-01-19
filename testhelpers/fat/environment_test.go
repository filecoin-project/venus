package fat

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	iptb "github.com/ipfs/iptb/testbed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestEnvironmentSetupAndTeardown(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx := context.Background()

	testDir, err := ioutil.TempDir(".", "environmentTest")
	require.NoError(err)
	defer os.RemoveAll(testDir)

	env, err := NewEnvironment(testDir)
	assert.NoError(err)
	assert.NotNil(env)
	assert.Equal(testDir, env.Location)

	// did we create the dir correctly?
	_, err = os.Stat(env.Location)
	assert.NoError(err)

	// did we teardown correctly?
	assert.NoError(env.Teardown(ctx))
	_, existsErr := os.Stat(env.Location)
	assert.True(os.IsNotExist(existsErr))
}

func TestProcessCreateAndTeardown(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx := context.Background()

	testDir, err := ioutil.TempDir(".", "environmentTest")
	require.NoError(err)
	defer os.RemoveAll(testDir)

	env, err := NewEnvironment(testDir)
	require.NoError(err)

	p, err := env.NewProcess(ctx, mockplugin.PluginName, nil)
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
}
