package fast

import (
	"context"
	"io/ioutil"
	"testing"

	iptb "github.com/ipfs/iptb/testbed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	mockplugin "github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin/mock"
)

func TestStartLogCapture(t *testing.T) {
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
	err = mfc.setupStderrCapturing()
	require.NoError(t, err)

	t.Run("test capture logs", func(t *testing.T) {
		capture, err := mfc.StartLogCapture()
		require.NoError(t, err)

		_, err = mfc.RunCmdWithStdin(ctx, nil, "add-to-daemonstderr", "hello")
		require.NoError(t, err)

		err = mfc.lp.Pull()
		require.NoError(t, err)

		capture.Stop()

		bb, err := ioutil.ReadAll(capture)
		require.NoError(t, err)

		require.Equal(t, "hello\n", string(bb))
	})

	err = mfc.teardownStderrCapturing()
	require.NoError(t, err)
}
