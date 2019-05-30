package migration12_test

import (
	"testing"

	"github.com/magiconair/properties/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/repo"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/migration/internal"
	migration12 "github.com/filecoin-project/go-filecoin/tools/migration/migrations/repo-1-2"
)

func TestDescribe(t *testing.T) {
	tf.UnitTest(t)

	container, _ := internal.RequireInitRepo(t, 1)
	defer repo.RequireRemoveAll(t, container)

	mig := migration12.MetadataFormatJSONtoCBOR{}

	expected := `MetadataFormatJSONtoCBOR migrates the storage repo from version 1 to 2.

    This migration changes the chain store metadata serialization from JSON to CBOR.
    The chain store metadata will be read in as JSON and rewritten as CBOR. 
	Chain store metadata consists of CIDs and the chain State Root. 
	No other repo data is changed.  Migrations are performed on a copy of the
	chain store.
`
	assert.Equal(t, expected, mig.Describe())
}

func TestMigrateEmptyRepo(t *testing.T) {
	tf.UnitTest(t)

	mig := migration12.MetadataFormatJSONtoCBOR{}
	oldVer, newVer := mig.Versions()

	container, repoSymLink := internal.RequireInitRepo(t, oldVer)
	defer repo.RequireRemoveAll(t, container)

	newRepoPath, err := internal.CloneRepo(repoSymLink, newVer)
	require.NoError(t, err)
	defer repo.RequireRemoveAll(t, newRepoPath)

	err = mig.Migrate(newRepoPath)
	require.NoError(t, err)
}
