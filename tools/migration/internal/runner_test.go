package internal_test

import (
	"os"
	"testing"

	ast "github.com/stretchr/testify/assert"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	. "github.com/filecoin-project/go-filecoin/tools/migration/internal"
)

// TODO: Issue #2595 Implement first repo migration
func TestMigrationRunner_Run(t *testing.T) {
	tf.UnitTest(t)
	assert := ast.New(t)

	rmh := TestRepoHelper{"/tmp/migration_test_runner_old", "/tmp/migration_test_runner_new"}
	runner := NewMigrationRunner(false, "describe", &rmh)
	assert.NoError(runner.Run())
}

type TestRepoHelper struct {
	oldRepoPath, newRepoPath string
}

func (trh *TestRepoHelper) GetOldRepo() (*os.File, error) {
	panic("not implemented")
}

func (trh *TestRepoHelper) MakeNewRepo() (*os.File, error) {
	panic("not implemented")
}
