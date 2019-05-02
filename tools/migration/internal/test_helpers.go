package internal

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// RequireMakeTempDir ensures that a temporary directory is created
func RequireMakeTempDir(t *testing.T, dirname string) string {
	newdir, err := ioutil.TempDir("", dirname)
	require.NoError(t, err)
	return newdir
}

// RequireRmDir ensures that the error condition is checked when we clean up
// after creating a temporary directory.
func RequireRmDir(t *testing.T, dirname string) {
	require.NoError(t, os.RemoveAll(dirname))
}
