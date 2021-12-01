package typeutil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExportedFields(t *testing.T) {
	f := ExportedFields(new(int))
	require.Nil(t, f, "nil fields for non-struct type")

	type S struct {
		A  int
		B  uint
		C  bool
		d  float32 // nolint
		_C float64 // nolint
	}

	f = ExportedFields(S{})
	require.Len(t, f, 3, "exported fields")
}
