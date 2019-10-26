package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/filecoin-project/go-filecoin/build/project"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
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
		assert.Contains(t, string(out), "Error: invalid command: foo")
		assert.Contains(t, string(out), usage)
		assert.Error(t, err)
	})

	t.Run("accepts --verbose or -v with valid command", func(t *testing.T) {
		repoDir, symlink := internal.RequireInitRepo(t, 1)
		defer repo.RequireRemoveAll(t, repoDir)
		defer repo.RequireRemoveAll(t, symlink)

		expected := "MetadataFormatJSONtoCBOR migrates the storage repo from version 1 to 2."

		out, err := exec.Command(command, "describe", "--old-repo="+symlink, "--verbose").CombinedOutput()
		assert.NoError(t, err)
		assert.Contains(t, string(out), expected)

		_, err = exec.Command(command, "describe", "--old-repo="+symlink, "-v").CombinedOutput()
		assert.NoError(t, err)
		assert.Contains(t, string(out), expected)
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
	root := project.Root()

	bin := filepath.Join(root, "tools/migration/go-filecoin-migrate")
	_, err := os.Stat(bin)
	require.NoError(t, err)

	return bin
}
