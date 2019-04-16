package migrate_1_to_2_test

import (
	int "github.com/filecoin-project/go-filecoin/tools/migration/internal"
	. "github.com/filecoin-project/go-filecoin/tools/migration/migrate_1-to-2"
	req "github.com/stretchr/testify/require"
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

// TODO: Issue 2595 , complete these tests
func TestMigrator_Describe(t *testing.T) {
	require := req.New(t)
	migl := requireMakeMigl(require)
	t.Run("describe outputs some stuff", func(t *testing.T) {
		nm := NewMigrator_1_2(migl)
		nm.Describe()
	})

}

func TestMigrator_Migrate(t *testing.T) {
}

func TestMigrator_Validate(t *testing.T) {
}

func requireMakeMigl(require *req.Assertions) *int.Migl {
	logfile, err := os.Create("/tmp/foo.txt") // truncates
	require.NoError(err)
	migl := int.NewMigl(logfile, true)
	return &migl

}
