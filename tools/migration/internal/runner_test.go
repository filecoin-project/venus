package internal_test

import (
	"testing"

	ast "github.com/stretchr/testify/assert"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	. "github.com/filecoin-project/go-filecoin/tools/migration/internal"
)

// TODO: Issue #2595 Implement first repo migration
func TestMigrationRunner_Run(t *testing.T) {
	tf.UnitTest(t)
	assert := ast.New(t)

	runner := NewMigrationRunner(false, "describe", "1", "2")
	assert.NoError(runner.Run())
}
