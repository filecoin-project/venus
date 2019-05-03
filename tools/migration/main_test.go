package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/migration/internal"
)

func TestUsage(t *testing.T) {
	tf.IntegrationTest(t) // because we're using exec.Command
	command := requireGetMigrationBinary(t)
	usage := `go-filecoin-migrate (describe|buildonly|migrate) --old-repo=<repolink> [-h|--help] [-v|--verbose]`

	t.Run("bare invocation prints usage but exits with 1", func(t *testing.T) {
		out, err := exec.Command(command).CombinedOutput()
		assert.Contains(t, string(out), usage)
		assert.Error(t, err)
	})

	t.Run("-h prints usage", func(t *testing.T) {
		out, err := exec.Command(command, "-h").CombinedOutput()
		assert.Contains(t, string(out), usage)
		assert.NoError(t, err)
	})

	t.Run("--help prints usage", func(t *testing.T) {
		out, err := exec.Command(command, "--help").CombinedOutput()
		assert.Contains(t, string(out), usage)
		assert.NoError(t, err)
	})
}

func TestOptions(t *testing.T) {
	tf.IntegrationTest(t) // because we're using exec.Command
	command := requireGetMigrationBinary(t)
	usage := `go-filecoin-migrate (describe|buildonly|migrate) --old-repo=<repolink> [-h|--help] [-v|--verbose]`

	t.Run("error when calling with invalid command", func(t *testing.T) {
		out, err := exec.Command(command, "foo", "--old-repo=something").CombinedOutput()
		assert.Contains(t, string(out), "Error: Invalid command: foo")
		assert.Contains(t, string(out), usage)
		assert.Error(t, err)
	})

	t.Run("accepts --verbose or -v with valid command", func(t *testing.T) {
		repoDir, symlink := internal.RequireSetupTestRepo(t, 0)
		defer internal.RequireRemove(t, repoDir)
		defer internal.RequireRemove(t, symlink)

		// TODO: this will break once there is a valid migration, but we can't mock migrations
		//       here because we're running the CLI.
		out, err := exec.Command(command, "describe", "--old-repo="+symlink, "--verbose").CombinedOutput()
		assert.Error(t, err)
		assert.Equal(t, "binary version 0 = repo version 0; migration not run\n", string(out))

		_, err = exec.Command(command, "describe", "--old-repo="+symlink, "-v").CombinedOutput()
		assert.Error(t, err)
		assert.Equal(t, "binary version 0 = repo version 0; migration not run\n", string(out))
	})

	t.Run("requires --old-repo argument", func(t *testing.T) {
		out, err := exec.Command(command, "describe").CombinedOutput()
		expected := "Error: --old-repo is required"
		assert.Error(t, err)
		assert.Contains(t, string(out), expected) // should include describe output when implemented
		assert.Contains(t, string(out), usage)
	})
}

func requireGetMigrationBinary(t *testing.T) string {
	gopath, err := testhelpers.GetGoPath()
	require.NoError(t, err)

	bin := filepath.Join(gopath, "/src/github.com/filecoin-project/go-filecoin/tools/migration/go-filecoin-migrate")
	_, err = os.Stat(bin)
	require.NoError(t, err)

	return bin
}
