package typeutil

import (
	"testing"

	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/stretchr/testify/require"
)

func TestExportedFields(t *testing.T) {
	tf.UnitTest(t)

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
