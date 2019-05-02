package fastesting

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

type tlogWriter struct {
	buf bytes.Buffer
}

func (w *tlogWriter) Logf(format string, args ...interface{}) {
	fmt.Fprintf(&w.buf, format, args...)
}

func TestLogWriter(t *testing.T) {
	tf.UnitTest(t)

	input := []string{
		"line1\n",
		"line2\n",
		"line3\n",
		"line4\n",
		"line5\n",
	}

	out := &tlogWriter{}
	lw := newLogWriter(out)

	for _, line := range input {
		_, err := lw.Write([]byte(fmt.Sprintf("%s", line)))
		require.NoError(t, err)
	}

	require.NoError(t, lw.Close())

	require.Equal(t, strings.Join(input, ""), out.buf.String())
}
