package fast

import (
	"context"
	"io/ioutil"
	"testing"

	iptb "github.com/ipfs/iptb/testbed"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"

	mockplugin "github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin/mock"
)

func TestStartLogCapture(t *testing.T) {
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
	mfc.setupStderrCapturing()
	require.NoError(err)

	t.Run("test capture logs", func(t *testing.T) {
		capture, err := mfc.StartLogCapture()
		require.NoError(err)

		_, err = mfc.RunCmdWithStdin(ctx, nil, "add-to-daemonstderr", "hello")
		require.NoError(err)

		err = mfc.lp.Pull()
		require.NoError(err)

		capture.Stop()

		bb, err := ioutil.ReadAll(capture)
		require.NoError(err)

		require.Equal("hello\n", string(bb))
	})

	mfc.teardownStderrCapturing()
	require.NoError(err)
}
