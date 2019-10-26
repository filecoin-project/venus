package fastutil

import (
	"bytes"
	"errors"
	"io"
	"testing"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/stretchr/testify/require"
)

func TestLinePuller(t *testing.T) {
	tf.UnitTest(t)

	t.Run("pull on empty source", func(t *testing.T) {
		var source bytes.Buffer
		var sink bytes.Buffer

		lp := NewLinePuller(&source, &sink)
		err := lp.Pull()
		require.NoError(t, err)
	})

	t.Run("pull one line", func(t *testing.T) {
		var source bytes.Buffer
		var sink bytes.Buffer

		lp := NewLinePuller(&source, &sink)

		source.WriteString("Filecoin\n")

		err := lp.Pull()
		require.NoError(t, err)

		require.Equal(t, "Filecoin\n", sink.String())

	})

	t.Run("pull many lines", func(t *testing.T) {
		var source bytes.Buffer
		var sink bytes.Buffer
		var expected bytes.Buffer

		lp := NewLinePuller(&source, &sink)

		require.NoError(t, writeLines(0, 1000, &source, &expected))

		err := lp.Pull()
		require.NoError(t, err)

		compare(t, expected.Bytes(), sink.Bytes())
	})

	t.Run("pull after EOF", func(t *testing.T) {
		var source manualReader
		var sink bytes.Buffer
		var expected bytes.Buffer

		lp := NewLinePuller(&source, &sink)

		source.bytes = []byte("Hello World\n")
		source.err = io.EOF

		expected.Write(source.bytes)

		err := lp.Pull()
		require.NoError(t, err)

		expected.Write(source.bytes)

		err = lp.Pull()
		require.NoError(t, err)

		compare(t, expected.Bytes(), sink.Bytes())
	})

	t.Run("source returns error", func(t *testing.T) {
		var source manualReader
		var sink bytes.Buffer
		var expected bytes.Buffer

		lp := NewLinePuller(&source, &sink)

		source.bytes = []byte{}
		source.err = errors.New("An error")

		err := lp.Pull()
		require.Equal(t, err, source.err)

		compare(t, expected.Bytes(), sink.Bytes())
	})
}

type manualReader struct {
	bytes []byte
	err   error
}

func (r *manualReader) Read(p []byte) (int, error) {
	if len(p) < len(r.bytes) {
		panic("manualReader bytes is larger than read buffer")
	}

	return copy(p, r.bytes), r.err
}
