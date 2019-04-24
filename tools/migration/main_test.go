package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	ast "github.com/stretchr/testify/assert"
	req "github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/testhelpers"
)

func TestUsage(t *testing.T) {
	tf.IntegrationTest(t) // because we're using exec.Command
	require := req.New(t)
	assert := ast.New(t)
	command := mustGetMigrationBinary(require)
	expected := "go-filecoin-migrate (describe|buildonly|migrate) [-h|--help][-v|--verbose][--old-repo=<repodir>][--new-repo=<newrepo-prefix]"

	t.Run("bare invocation prints usage but exits with 1", func(t *testing.T) {
		out, err := exec.Command(command).CombinedOutput()
		assert.Contains(string(out), expected)
		assert.Error(err)
	})

	t.Run("-h prints usage", func(t *testing.T) {
		out, err := exec.Command(command, "-h").CombinedOutput()
		assert.Contains(string(out), expected)
		assert.NoError(err)
	})

	t.Run("--help prints usage", func(t *testing.T) {
		out, err := exec.Command(command, "--help").CombinedOutput()
		assert.Contains(string(out), expected)
		assert.NoError(err)
	})
}

func TestOptions(t *testing.T) {
	tf.IntegrationTest(t) // because we're using exec.Command
	require := req.New(t)
	assert := ast.New(t)
	command := mustGetMigrationBinary(require)
	usage := "go-filecoin-migrate (describe|buildonly|migrate) [-h|--help][-v|--verbose][--old-repo=<repodir>][--new-repo=<newrepo-prefix]"

	t.Run("error when calling with invalid command", func(t *testing.T) {
		out, err := exec.Command(command, "foo").CombinedOutput()
		assert.Contains(string(out), "Error: Invalid command: foo")
		assert.Contains(string(out), usage)
		assert.Error(err)
	})

	t.Run("accepts --verbose with valid command", func(t *testing.T) {
		out, err := exec.Command(command, "describe", "--verbose").CombinedOutput()
		assert.Contains(string(out), "") // should include describe output when implemented
		assert.NoError(err)
	})

	t.Run("accepts -v with valid command", func(t *testing.T) {
		out, err := exec.Command(command, "describe", "--verbose").CombinedOutput()
		assert.Contains(string(out), "") // should include describe output when implemented
		assert.NoError(err)
	})
}

func mustGetMigrationBinary(require *req.Assertions) string {
	gopath, err := testhelpers.GetGoPath()
	require.NoError(err)

	bin := filepath.Join(gopath, "/src/github.com/filecoin-project/go-filecoin/tools/migration/go-filecoin-migrate")
	_, err = os.Stat(bin)
	require.NoError(err)

	return bin
}
