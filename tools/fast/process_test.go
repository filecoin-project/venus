package fast

import (
	"context"
	"io"
	"io/ioutil"
	"testing"
	"time"

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

func mustGetStdout(require *require.Assertions, out io.ReadCloser) string {
	o, err := ioutil.ReadAll(out)
	require.NoError(err)
	return string(o)
}

type testJSONOutParam struct {
	Key string
}

// A sanity test to ensure returning strings, json, and ldjson works as expected.
func TestRunCmds(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx := context.Background()
	dir := "mockdir"

	ns := iptb.NodeSpec{
		Type:  mockplugin.PluginName,
		Dir:   dir,
		Attrs: nil,
	}

	c, err := ns.Load()
	assert.NoError(err)

	fc, ok := c.(IPTBCoreExt)
	require.True(ok)

	mfc := NewFilecoinProcess(ctx, fc, EnvironmentOpts{})

	t.Run("test RunCmdWithStdin", func(t *testing.T) {
		out, err := mfc.RunCmdWithStdin(ctx, nil, "")
		require.NoError(err)
		outStr := mustGetStdout(require, out.Stdout())
		assert.Equal("string", outStr)
	})

	t.Run("test RunCmdJSONWithStdin", func(t *testing.T) {
		var outParam testJSONOutParam
		err = mfc.RunCmdJSONWithStdin(ctx, nil, &outParam, "json")
		require.NoError(err)
		assert.Equal("value", outParam.Key)
	})

	t.Run("test RunCmdLDJsonWithStdin", func(t *testing.T) {
		var outLdParam testJSONOutParam
		cmdDecoder, err := mfc.RunCmdLDJSONWithStdin(ctx, nil, "ldjson")
		require.NoError(err)
		assert.NoError(cmdDecoder.Decode(&outLdParam))
		assert.Equal("value1", outLdParam.Key)
		assert.NoError(cmdDecoder.Decode(&outLdParam))
		assert.Equal("value2", outLdParam.Key)
	})
}

func TestInitDaemon(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx := context.Background()
	dir := "mockdir"

	ns := iptb.NodeSpec{
		Type:  mockplugin.PluginName,
		Dir:   dir,
		Attrs: nil,
	}

	c, err := ns.Load()
	assert.NoError(err)

	fc, ok := c.(IPTBCoreExt)
	require.True(ok)

	t.Run("providing both InitDaemon options and environment options", func(t *testing.T) {

		fastenvOpts := EnvironmentOpts{
			InitOpts: []ProcessInitOption{POGenesisFile("http://example.com/genesis.car")},
		}

		mfc := NewFilecoinProcess(ctx, fc, fastenvOpts)
		_, err := mfc.InitDaemon(context.Background(), "--foo")
		require.Equal(ErrDoubleInitOpts, err)
	})
}

func TestStartDaemon(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx := context.Background()
	dir := "mockdir"

	ns := iptb.NodeSpec{
		Type:  mockplugin.PluginName,
		Dir:   dir,
		Attrs: nil,
	}

	c, err := ns.Load()
	assert.NoError(err)

	fc, ok := c.(IPTBCoreExt)
	require.True(ok)

	t.Run("providing both InitDaemon options and environment options", func(t *testing.T) {

		fastenvOpts := EnvironmentOpts{
			DaemonOpts: []ProcessDaemonOption{POBlockTime(time.Second)},
		}

		mfc := NewFilecoinProcess(ctx, fc, fastenvOpts)
		_, err := mfc.StartDaemon(context.Background(), true, "--foo")
		require.Equal(ErrDoubleDaemonOpts, err)
	})
}
