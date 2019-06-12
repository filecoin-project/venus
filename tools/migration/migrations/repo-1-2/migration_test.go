package migration12_test

import (
	"context"
	"os"
	"os/exec"
	"path"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/repo"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/migration/internal"
	migration12 "github.com/filecoin-project/go-filecoin/tools/migration/migrations/repo-1-2"
)

// ==============  IMPORTANT ================
// PLEASE SEE THE README IF YOU ARE HERE BECAUSE YOUR CHANGES BROKE A MIGRATION TEST
// ==========================================
func TestDescribe(t *testing.T) {
	tf.UnitTest(t)

	container, _ := internal.RequireInitRepo(t, 1)
	defer repo.RequireRemoveAll(t, container)

	mig := migration12.MetadataFormatJSONtoCBOR{}

	expected := `MetadataFormatJSONtoCBOR migrates the storage repo from version 1 to 2.

    This migration changes the chain store metadata serialization from JSON to CBOR.
    The chain store metadata will be read in as JSON and rewritten as CBOR. 
	Chain store metadata consists of associations between tipset keys and state 
	root cids and the tipset key of the head of the chain.
	No other repo data is changed.  Migrations are performed on a copy of the
	chain store.
`
	assert.Equal(t, expected, mig.Describe())
}

func TestMigrateSomeRepo(t *testing.T) {
	tf.UnitTest(t)

	mig := migration12.MetadataFormatJSONtoCBOR{}
	_, newVer := mig.Versions()

	curDir, err := syscall.Getwd()
	require.NoError(t, err)

	// The test fixture was created by running FAST's localnet script with a long block time,
	// allowing it to mine once, suspending it to copy the filecoin repo, then killing the script
	// (because it deletes the repo afterward).
	// It was then stripped of everything that did not apply to this migration, tarred and gzipped.
	// This guarantees that we have an example repo in an unmigrated, pristine v1 state with
	// a minimal footprint.
	fixturePath := path.Join(curDir, "fixtures")
	fixtureTarball := path.Join(fixturePath, "repo-1.tgz")

	// unpack the tarball
	cmd := exec.CommandContext(context.Background(), "tar", "xzf", fixtureTarball)
	require.NoError(t, cmd.Run())
	defer repo.RequireRemoveAll(t, path.Join(curDir, "repo-1"))

	// make a symlink
	repoDir := path.Join(curDir, "repo-1", "repo-20190612-152655-v001")
	repoSymLink := path.Join(curDir, "repo-1", "repo")
	require.NoError(t, os.Symlink(repoDir, repoSymLink))

	t.Run("Happy path: valid migration passes validation", func(t *testing.T) {
		newRepoPath, err := internal.CloneRepo(repoSymLink, newVer)
		require.NoError(t, err)
		defer repo.RequireRemoveAll(t, newRepoPath)

		require.NoError(t, mig.Migrate(newRepoPath))

		err = mig.Validate(repoSymLink, newRepoPath)
		require.NoError(t, err)
	})

	t.Run("Validation before migration is run fails validation", func(t *testing.T) {
		newRepoPath, err := internal.CloneRepo(repoSymLink, newVer)
		require.NoError(t, err)
		defer repo.RequireRemoveAll(t, newRepoPath)

		err = mig.Validate(repoSymLink, newRepoPath)
		require.Error(t, err)
		// Because Validate tried to load JSON encoded data as CBOR
		assert.Contains(t, err.Error(), "cbor: decoding rejected oversized byte field")
	})
}
