package typecheck

import (
	"io"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExportedMethods(t *testing.T) {

	meths, err := ExportedMethods(reflect.TypeOf((*io.ReadCloser)(nil)).Elem())
	require.NoError(t, err, "get exported methods for io.ReadCloser")
	require.Len(t, meths, 2, "exported methods for io.ReadCloser")

	var ci codecInt
	meths, err = ExportedMethods(&ci)
	require.NoError(t, err, "get exported methods for *codecInt")
	require.Len(t, meths, 8, "exported methods for *codecInt")

	meths, err = ExportedMethods(ci)
	require.NoError(t, err, "get exported methods for codecInt")
	require.Len(t, meths, 4, "exported methods for codecInt")
}
