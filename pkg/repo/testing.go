package repo

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// RequireMakeTempDir ensures that a temporary directory is created
func RequireMakeTempDir(t *testing.T, dirname string) string {
	newdir, err := os.MkdirTemp("", dirname)
	require.NoError(t, err)
	return newdir
}

// RequireOpenTempFile is a shortcut for opening a given temp file with a given
// suffix, then returning both a filename and a file pointer.
func RequireOpenTempFile(t *testing.T, suffix string) (*os.File, string) {
	file, err := os.CreateTemp("", suffix)
	require.NoError(t, err)
	name := file.Name()
	return file, name
}

// RequireReadLink reads a symlink that is expected to resolve successfully.
func RequireReadLink(t *testing.T, path string) string {
	target, err := os.Readlink(path)
	require.NoError(t, err)
	return target
}
