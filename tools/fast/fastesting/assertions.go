package fastesting

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/tools/fast"
)

// AssertStdErrContains verifies that the last command stderr of 'fast' contains the
// string 'expected'
func AssertStdErrContains(t *testing.T, fast *fast.Filecoin, expected string) {
	var cmdOutBytes []byte
	w := bytes.NewBuffer(cmdOutBytes)
	written, err := io.Copy(w, fast.LastCmdStdErr())
	require.NoError(t, err)
	require.True(t, written > 0)
	assert.Contains(t, string(w.Bytes()), expected)
}
