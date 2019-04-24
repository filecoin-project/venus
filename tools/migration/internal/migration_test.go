package internal_test

import (
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	. "github.com/filecoin-project/go-filecoin/tools/migration/internal"
	"os"
	"testing"

	ast "github.com/stretchr/testify/assert"
)

func TestNewMigrationRunner(t *testing.T) {
	tf.UnitTest(t)
	assert := ast.New(t)

	rmh := TestRepoHelper{}

	assert.NotNil(NewMigrationRunner(false, "describe", &rmh))
}

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
