package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	ast "github.com/stretchr/testify/assert"
	req "github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/testhelpers"
)

func TestUsage(t *testing.T) {
	require := req.New(t)
	assert := ast.New(t)
	command := mustGetMigrationBinary(require)
	expected := "Usage:  go-filecoin-migrate (describe|buildonly|migrate) [--verbose]"

	t.Run("bare command prints usage", func(t *testing.T) {
		out, err := exec.Command(command).CombinedOutput()
		assert.Contains(string(out), expected)
		assert.NoError(err)
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
	require := req.New(t)
	assert := ast.New(t)
	command := mustGetMigrationBinary(require)

	t.Run("error when calling with invalid command", func(t *testing.T) {
		out, err := exec.Command(command, "foo").CombinedOutput()
		assert.Contains(string(out), "Error: Invalid command: foo")
		assert.Error(err)
	})

	t.Run("accepts --verbose with valid command", func(t *testing.T) {
		out, err := exec.Command(command, "describe", "--verbose").CombinedOutput()
		assert.Contains(string(out), "Migration from 0.1 to 0.2: a test migrator that just updates the repo version")
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
