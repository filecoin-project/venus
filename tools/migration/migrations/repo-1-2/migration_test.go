package migration12_test

import (
	"context"
	"os"
	"os/exec"
	"path"
	"syscall"
	"testing"

	"github.com/magiconair/properties/assert"
	"github.com/stretchr/testify/require"
	"gotest.tools/env"

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

func TestMigrateSomeRepo(t *testing.T) {
	tf.UnitTest(t)

	mig := migration12.MetadataFormatJSONtoCBOR{}
	_, newVer := mig.Versions()

	curDir, err := syscall.Getwd() // this will be the top go-filecoin dir
	require.NoError(t, err)

	fixtureTarball := path.Join(curDir, "tools", "migration", "fixtures", "repo-12.tgz")

	tmpDir := repo.RequireMakeTempDir(t, "")

	// unpack the tarball into a tmp dir
	exec.CommandContext(context.Background(), "tar", "xzf", tmpDir, fixtureTarball)

	// make a symlink
	repoSymLink := os.Link(path.Base(fixtureTarball)+"/repo-somethingsomething", "repo")

	newRepoPath, err := internal.CloneRepo(repoSymLink, newVer)
	require.NoError(t, err)
	defer repo.RequireRemoveAll(t, newRepoPath)

	err = mig.Migrate(newRepoPath)
	require.NoError(t, err)

	err = mig.Validate(repoSymLink, newRepoPath)
	require.NoError(t, err)
}
