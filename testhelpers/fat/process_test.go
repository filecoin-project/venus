package fat

import (
	"context"
	"io"
	"io/ioutil"
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

	mfc := NewFilecoinProcess(ctx, c)

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
