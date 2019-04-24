package internal

import (
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	ast "github.com/stretchr/testify/assert"
	req "github.com/stretchr/testify/require"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestGetNewRepoPath(t *testing.T) {
	tf.UnitTest(t)
	assert := ast.New(t)
	require := req.New(t)

	t.Run("Uses the new repo opt as a prefix if provided", func(t *testing.T) {
		dirname := "/tmp/mynew_repodir"
		newpath := getNewRepoPath("/tmp/myold_repodir", dirname, "1", "2")
		rgx, err := regexp.Compile("/tmp/mynew_repodir_1_2_[0-9]{4}-[0-9]{2}-[0-9]{2}_[0-9]{6}")
		require.NoError(err)
		assert.Regexp(rgx, newpath)
	})

	t.Run("Adds a timestamp to the new repo dir", func(t *testing.T) {
		dirname := "/tmp/myfilecoindir"
		newpath := getNewRepoPath(dirname, "", "1", "2")
		rgx, err := regexp.Compile("/tmp/myfilecoindir_1_2_[0-9]{4}-[0-9]{2}-[0-9]{2}_[0-9]{6}")
		require.NoError(err)
		assert.Regexp(rgx, newpath)
	})
}

func TestGetOldRepoPath(t *testing.T) {
	// technically it'll return what's at FIL_PATH if that is set,
	// but in testing this is set to the default.
	tf.UnitTest(t)
	assert := ast.New(t)

	home := os.Getenv("HOME")
	expected := strings.Join([]string{home, "/.filecoin"}, "")
	assert.Equal(expected, getOldRepoPath(""))
}
