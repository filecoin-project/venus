package fast

import (
	"context"
	"io"
	"io/ioutil"
	"testing"
	"time"

	iptb "github.com/ipfs/iptb/testbed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
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

func mustGetStdout(t *testing.T, out io.ReadCloser) string {
	o, err := ioutil.ReadAll(out)
	require.NoError(t, err)
	return string(o)
}

type testJSONOutParam struct {
	Key string
}

// A sanity test to ensure returning strings, json, and ldjson works as expected.
func TestRunCmds(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	dir := "mockdir"

	ns := iptb.NodeSpec{
		Type:  mockplugin.PluginName,
		Dir:   dir,
		Attrs: nil,
	}

	c, err := ns.Load()
	assert.NoError(t, err)

	fc, ok := c.(IPTBCoreExt)
	require.True(t, ok)

	mfc := NewFilecoinProcess(ctx, fc, FilecoinOpts{})

	t.Run("test RunCmdWithStdin", func(t *testing.T) {
		out, err := mfc.RunCmdWithStdin(ctx, nil, "")
		require.NoError(t, err)
		outStr := mustGetStdout(t, out.Stdout())
		assert.Equal(t, "string", outStr)
	})

	t.Run("test RunCmdJSONWithStdin", func(t *testing.T) {
		var outParam testJSONOutParam
		err = mfc.RunCmdJSONWithStdin(ctx, nil, &outParam, "json")
		require.NoError(t, err)
		assert.Equal(t, "value", outParam.Key)
	})

	t.Run("test RunCmdLDJsonWithStdin", func(t *testing.T) {
		var outLdParam testJSONOutParam
		cmdDecoder, err := mfc.RunCmdLDJSONWithStdin(ctx, nil, "ldjson")
		require.NoError(t, err)
		assert.NoError(t, cmdDecoder.Decode(&outLdParam))
		assert.Equal(t, "value1", outLdParam.Key)
		assert.NoError(t, cmdDecoder.Decode(&outLdParam))
		assert.Equal(t, "value2", outLdParam.Key)
	})
}

func TestInitDaemon(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	dir := "mockdir"

	ns := iptb.NodeSpec{
		Type:  mockplugin.PluginName,
		Dir:   dir,
		Attrs: nil,
	}

	c, err := ns.Load()
	assert.NoError(t, err)

	fc, ok := c.(IPTBCoreExt)
	require.True(t, ok)

	t.Run("providing both InitDaemon options and environment options", func(t *testing.T) {

		fastenvOpts := FilecoinOpts{
			InitOpts: []ProcessInitOption{POGenesisFile("http://example.com/genesis.car")},
		}

		mfc := NewFilecoinProcess(ctx, fc, fastenvOpts)
		_, err := mfc.InitDaemon(context.Background(), "--foo")
		require.Equal(t, ErrDoubleInitOpts, err)
	})
}

func TestStartDaemon(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	dir := "mockdir"

	ns := iptb.NodeSpec{
		Type:  mockplugin.PluginName,
		Dir:   dir,
		Attrs: nil,
	}

	c, err := ns.Load()
	assert.NoError(t, err)

	fc, ok := c.(IPTBCoreExt)
	require.True(t, ok)

	t.Run("providing both InitDaemon options and environment options", func(t *testing.T) {

		fastenvOpts := FilecoinOpts{
			DaemonOpts: []ProcessDaemonOption{POBlockTime(time.Second)},
		}

		mfc := NewFilecoinProcess(ctx, fc, fastenvOpts)
		_, err := mfc.StartDaemon(context.Background(), true, "--foo")
		require.Equal(t, ErrDoubleDaemonOpts, err)
	})
}
