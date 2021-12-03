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

	type I interface {
		Public()
		private()
	}

	meths = AllMethods(reflect.TypeOf((*I)(nil)).Elem())
	require.Len(t, meths, 2, "all methods for I")

	meths = ExportedMethods(reflect.TypeOf((*I)(nil)).Elem())
	require.Len(t, meths, 1, "exported methods for I")

	var ci codecInt
	meths = ExportedMethods(&ci)
	require.Len(t, meths, 8, "exported methods for *codecInt")

	meths = ExportedMethods(ci)
	require.Len(t, meths, 4, "exported methods for codecInt")

	meths = AllMethods(&ci)
	require.Len(t, meths, 8, "all methods for *codecInt")

	meths = AllMethods(ci)
	require.Len(t, meths, 4, "all methods for codecInt")
}
