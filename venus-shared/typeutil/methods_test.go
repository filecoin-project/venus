package typeutil

import (
	"io"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExportedMethods(t *testing.T) {

	meths := ExportedMethods(reflect.TypeOf((*io.ReadCloser)(nil)).Elem())
	require.Len(t, meths, 2, "exported methods for io.ReadCloser")

	var ci codecInt
	meths = ExportedMethods(&ci)
	require.Len(t, meths, 8, "exported methods for *codecInt")

	meths = ExportedMethods(ci)
	require.Len(t, meths, 4, "exported methods for codecInt")
}
