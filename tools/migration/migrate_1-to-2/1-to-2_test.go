package migrate_1_to_2_test

import (
	"bytes"
	"github.com/filecoin-project/go-filecoin/tools/migration"
	. "github.com/filecoin-project/go-filecoin/tools/migration/migrate_1-to-2"
	ast "github.com/stretchr/testify/assert"
	req "github.com/stretchr/testify/require"
	"log"
	"os"
	"testing"
)

func TestNewMigrator_1_2(t *testing.T) {
	require := req.New(t)
	migl := requireMakeMigl(require)

	t.Run("sanity check instance", func(t *testing.T) {
		NewMigrator_1_2(migl).Describe()
	})

}

func TestMigrator_Describe(t *testing.T) {
	assert := ast.New(t)
	require := req.New(t)
	migl := requireMakeMigl(require)
	t.Run("describe outputs some stuff", func(t *testing.T) {

		nm := NewMigrator_1_2(migl)
		output := captureOutput(func() {
			nm.Describe()
		})
		assert.Equal("removed certificate www.example.com\n", output)

	})

}

func TestMigrator_Migrate(t *testing.T) {

}

func TestMigrator_Validate(t *testing.T) {

}

func requireMakeMigl(require *req.Assertions) *migration.Migl {
	logfile, err := os.Create("/tmp/foo.txt") // truncates
	require.NoError(err)
	migl := migration.NewMigl(logfile, true)
	return &migl

}

func captureOutput(f func()) string {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetOutput(os.Stderr)
	return buf.String()
}
