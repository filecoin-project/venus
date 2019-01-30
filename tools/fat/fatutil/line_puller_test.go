package fatutil

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLinePuller(t *testing.T) {
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

		writeLines(0, 1000, &source, &expected)

		err := lp.Pull()
		require.NoError(t, err)

		compare(t, expected.Bytes(), sink.Bytes())
	})
}
