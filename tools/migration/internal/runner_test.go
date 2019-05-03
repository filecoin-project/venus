package internal_test

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	. "github.com/filecoin-project/go-filecoin/tools/migration/internal"
)

// TODO: Issue #2595 Implement first repo migration
func TestMigrationRunner_Run(t *testing.T) {
	tf.UnitTest(t)
	dummyLogFile, err := ioutil.TempFile("", "logfile")
	require.NoError(t, err)
	logger := NewLogger(dummyLogFile, false)
	defer func() {
		require.NoError(t, os.Remove(dummyLogFile.Name()))
	}()
	runner := NewMigrationRunner(logger, "describe", "/home/filecoin-symlink")
	assert.NoError(t, runner.Run())
}
