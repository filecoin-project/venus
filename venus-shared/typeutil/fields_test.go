package typecheck

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExportedFields(t *testing.T) {
	_, err := ExportedFields(new(int))
	require.Error(t, err, "only allow structs")

	type S struct {
		A  int
		B  uint
		C  bool
		d  float32 // nolint
		_C float64 // nolint
	}

	f, err := ExportedFields(S{})
	require.NoError(t, err, "get exported fields")
	require.Len(t, f, 3, "exported fields")
}
