package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	req "github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/testhelpers"
)

func TestUsage(t *testing.T) {
	require := req.New(t)
	out, _ := exec.Command(mustGetMigrationBinary(require)).CombinedOutput()
	assert.Contains(t, string(out), "Usage:  go-filecoin-migrate (describe|buildonly|migrate) [--verbose]")
}

func TestNoCommand(t *testing.T) {

}

func TestOptions(t *testing.T) {

}

func mustGetMigrationBinary(require *req.Assertions) string {
	gopath, err := testhelpers.GetGoPath()
	require.NoError(err)

	bin := filepath.Join(gopath, "/src/github.com/filecoin-project/go-filecoin/tools/migration/go-filecoin-migrate")
	_, err = os.Stat(bin)
	require.NoError(err)

	return bin
}
