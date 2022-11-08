// stm: #unit
package repo

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	ds "github.com/ipfs/go-datastore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/pkg/config"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func TestInitRepoDirect(t *testing.T) {
	tf.UnitTest(t)
	cfg := config.NewDefaultConfig()

	// Inits a repo and opens it (ensuring it is openable)
	initAndOpenRepoDirect := func(repoPath string, version uint, cfg *config.Config) (*FSRepo, error) {
		// stm: @REPO_FSREPO_INIT_DIRECT_001
		if err := InitFSRepoDirect(repoPath, version, cfg); err != nil {
			return nil, err
		}
		return OpenFSRepo(repoPath, version)
	}

	t.Run("successfully creates when directory exists", func(t *testing.T) {
		dir := t.TempDir()

		_, err := initAndOpenRepoDirect(dir, 42, cfg)
		assert.NoError(t, err)
		checkNewRepoFiles(t, dir, 42)
	})

	t.Run("successfully creates when directory does not exist", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "nested")

		_, err := initAndOpenRepoDirect(dir, 42, cfg)
		assert.NoError(t, err)
		checkNewRepoFiles(t, dir, 42)
	})

	t.Run("fails with error if directory is not writeable", func(t *testing.T) {
		// make read only dir
		dir := filepath.Join(t.TempDir(), "readonly")
		err := os.Mkdir(dir, 0o444)
		assert.NoError(t, err)
		assert.False(t, ConfigExists(dir))

		_, err = initAndOpenRepoDirect(dir, 42, cfg)
		assert.Contains(t, err.Error(), "permission")
	})

	t.Run("fails with error if directory not empty", func(t *testing.T) {
		dir := t.TempDir()

		err := os.WriteFile(filepath.Join(dir, "hi"), []byte("hello"), 0o644)
		assert.NoError(t, err)

		_, err = initAndOpenRepoDirect(dir, 42, cfg)
		assert.Contains(t, err.Error(), "empty")
	})
}

func TestFSRepoOpen(t *testing.T) {
	tf.UnitTest(t)

	t.Run("[fail] repo version newer than binary", func(t *testing.T) {
		repoPath := path.Join(t.TempDir(), "repo")

		// stm: @REPO_FSREPO_INIT_001
		assert.NoError(t, InitFSRepo(repoPath, 1, config.NewDefaultConfig()))
		// set wrong version
		// stm：@REPO_FSREPO_READ_VERSION_001
		assert.NoError(t, WriteVersion(repoPath, 99))

		_, err := OpenFSRepo(repoPath, 1)
		expected := fmt.Sprintf("binary needs update to handle repo version, got 99 expected %d. Update binary to latest release", LatestVersion)
		assert.EqualError(t, err, expected)
	})

	t.Run("[fail] version corrupt", func(t *testing.T) {
		repoPath := path.Join(t.TempDir(), "repo")

		assert.NoError(t, InitFSRepo(repoPath, 1, config.NewDefaultConfig()))
		// set wrong version
		assert.NoError(t, os.WriteFile(filepath.Join(repoPath, versionFilename), []byte("v.8"), 0o644))

		_, err := OpenFSRepo(repoPath, 1)
		assert.EqualError(t, err, "failed to read version: strconv.ParseUint: parsing \"v.8\": invalid syntax")
	})
}

func TestFSRepoRoundtrip(t *testing.T) {
	tf.UnitTest(t)

	cfg := config.NewDefaultConfig()
	cfg.API.APIAddress = "foo" // testing that what we get back isnt just the default

	repoPath := path.Join(t.TempDir(), "repo")
	assert.NoError(t, InitFSRepo(repoPath, 42, cfg))

	r, err := OpenFSRepo(repoPath, 42)
	assert.NoError(t, err)

	assert.Equal(t, cfg, r.Config())
	assert.NoError(t, r.ChainDatastore().Put(context.Background(), ds.NewKey("beep"), []byte("boop")))
	assert.NoError(t, r.Close())

	r2, err := OpenFSRepo(repoPath, 42)
	assert.NoError(t, err)

	val, err := r2.ChainDatastore().Get(context.Background(), ds.NewKey("beep"))
	assert.NoError(t, err)
	assert.Equal(t, []byte("boop"), val)

	assert.NoError(t, r2.Close())
}

func TestFSRepoReplaceAndSnapshotConfig(t *testing.T) {
	tf.UnitTest(t)

	repoPath := path.Join(t.TempDir(), "repo")

	cfg := config.NewDefaultConfig()
	cfg.API.APIAddress = "foo"
	assert.NoError(t, InitFSRepo(repoPath, 42, cfg))

	expSnpsht, err := os.ReadFile(filepath.Join(repoPath, configFilename))
	require.NoError(t, err)

	r1, err := OpenFSRepo(repoPath, 42)
	assert.NoError(t, err)

	newCfg := config.NewDefaultConfig()
	newCfg.API.APIAddress = "bar"

	// stm: @REPO_FSREPO_REPLACE_CONFIG_001, @REPO_FSREPO_SNAPSHOT_CONFIG_001
	assert.NoError(t, r1.ReplaceConfig(newCfg))
	assert.Equal(t, "bar", r1.Config().API.APIAddress)
	// stm: REPO_FSREPO_CLOSE_001
	assert.NoError(t, r1.Close())

	r2, err := OpenFSRepo(repoPath, 42)
	assert.NoError(t, err)
	assert.Equal(t, "bar", r2.Config().API.APIAddress)
	assert.NoError(t, r2.Close())

	// assert that a single snapshot was created when replacing the config
	// get the snapshot file name
	snpFiles := getSnapshotFilenames(t, filepath.Join(repoPath, snapshotStorePrefix))
	require.Equal(t, 1, len(snpFiles))

	snpsht, err := os.ReadFile(filepath.Join(repoPath, snapshotStorePrefix, snpFiles[0]))
	require.NoError(t, err)
	assert.Equal(t, string(expSnpsht), string(snpsht))
}

func TestRepoLock(t *testing.T) {
	tf.UnitTest(t)

	repoPath := path.Join(t.TempDir(), "repo")

	cfg := config.NewDefaultConfig()
	assert.NoError(t, InitFSRepo(repoPath, 42, cfg))

	// stm: @REPO_FSREPO_OPEN_REPO_001
	r, err := OpenFSRepo(repoPath, 42)
	assert.NoError(t, err)
	assert.FileExists(t, filepath.Join(repoPath, lockFile))

	// stm: @REPO_FSREPO_EXISTS_001
	exist, err := Exists(repoPath)
	assert.NoError(t, err)
	assert.True(t, exist)

	_, err = OpenFSRepo(repoPath, 42)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to take repo lock")
	assert.NoError(t, r.Close())

	_, err = os.Lstat(filepath.Join(repoPath, lockFile))
	assert.True(t, os.IsNotExist(err))
}

func TestRepoLockFail(t *testing.T) {
	tf.UnitTest(t)

	repoPath := path.Join(t.TempDir(), "repo")

	cfg := config.NewDefaultConfig()
	assert.NoError(t, InitFSRepo(repoPath, 42, cfg))

	// set invalid version, to make opening the repo fail
	assert.NoError(t,
		os.WriteFile(filepath.Join(repoPath, versionFilename), []byte("hello"), 0o644),
	)

	_, err := OpenFSRepo(repoPath, 42)
	assert.Error(t, err)

	_, err = os.Lstat(filepath.Join(repoPath, lockFile))
	assert.True(t, os.IsNotExist(err))
}

func TestRepoAPIFile(t *testing.T) {
	tf.UnitTest(t)

	t.Run("APIAddr returns last value written to API file", func(t *testing.T) {
		withFSRepo(t, func(r *FSRepo) {
			// stm: @REPO_FSREPO_SET_API_ADDRESS
			mustSetAPIAddr(t, r, "/ip4/127.0.0.1/tcp/1234")

			rpcAPI := mustGetAPIAddr(t, r)
			assert.Equal(t, "/ip4/127.0.0.1/tcp/1234", rpcAPI)

			mustSetAPIAddr(t, r, "/ip4/127.0.0.1/tcp/4567")

			rpcAPI = mustGetAPIAddr(t, r)
			assert.Equal(t, "/ip4/127.0.0.1/tcp/4567", rpcAPI)
		})
	})

	t.Run("SetAPIAddr is idempotent", func(t *testing.T) {
		withFSRepo(t, func(r *FSRepo) {
			mustSetAPIAddr(t, r, "/ip4/127.0.0.1/tcp/1234")

			rpcAPI := mustGetAPIAddr(t, r)
			assert.Equal(t, "/ip4/127.0.0.1/tcp/1234", rpcAPI)

			mustSetAPIAddr(t, r, "/ip4/127.0.0.1/tcp/1234")

			rpcAPI = mustGetAPIAddr(t, r)
			assert.Equal(t, "/ip4/127.0.0.1/tcp/1234", rpcAPI)
		})
	})

	t.Run("APIAddr fails if called before SetAPIAddr", func(t *testing.T) {
		withFSRepo(t, func(r *FSRepo) {
			addr, err := r.APIAddr()
			assert.Error(t, err)
			assert.Equal(t, "", addr)
		})
	})

	t.Run("Close deletes API file", func(t *testing.T) {
		withFSRepo(t, func(r *FSRepo) {
			mustSetAPIAddr(t, r, "/ip4/127.0.0.1/tcp/1234")

			info, err := os.Stat(filepath.Join(r.path, apiFile))
			assert.NoError(t, err)
			assert.Equal(t, apiFile, info.Name())

			require.NoError(t, r.Close())

			_, err = os.Stat(filepath.Join(r.path, apiFile))
			assert.Error(t, err)
		})
	})

	t.Run("Close will succeed in spite of missing API file", func(t *testing.T) {
		withFSRepo(t, func(r *FSRepo) {
			mustSetAPIAddr(t, r, "/ip4/127.0.0.1/tcp/1234")

			err := os.Remove(filepath.Join(r.path, apiFile))
			assert.NoError(t, err)

			assert.NoError(t, r.Close())
		})
	})

	t.Run("SetAPI fails if unable to create API file", func(t *testing.T) {
		withFSRepo(t, func(r *FSRepo) {
			// create a file with permission bits that prevent us from truncating
			err := os.WriteFile(filepath.Join(r.path, apiFile), []byte("/ip4/127.0.0.1/tcp/9999"), 0o000)
			assert.NoError(t, err)

			// try to os.Create to same path - will see a failure
			err = r.SetAPIAddr("/ip4/127.0.0.1/tcp/1234")
			assert.Error(t, err)
		})
	})
}

func checkNewRepoFiles(t *testing.T, path string, version uint) {
	content, err := os.ReadFile(filepath.Join(path, configFilename))
	assert.NoError(t, err)

	t.Log("snapshot path was created during FSRepo Init")
	exists, err := fileExists(filepath.Join(path, snapshotStorePrefix))
	assert.NoError(t, err)
	assert.True(t, exists)

	// Asserting the exact content here is gonna get old real quick
	t.Log("config file matches expected value")
	config.SanityCheck(t, string(content))

	actualVersion, err := os.ReadFile(filepath.Join(path, versionFilename))
	assert.NoError(t, err)
	assert.Equal(t, strconv.FormatUint(uint64(version), 10), string(actualVersion))
}

func getSnapshotFilenames(t *testing.T, dir string) []string {
	files, err := os.ReadDir(dir)
	require.NoError(t, err)

	var snpFiles []string
	for _, f := range files {
		if strings.Contains(f.Name(), "snapshot") {
			snpFiles = append(snpFiles, f.Name())
		}
	}
	return snpFiles
}

func withFSRepo(t *testing.T, f func(*FSRepo)) {
	dir := t.TempDir()

	cfg := config.NewDefaultConfig()
	require.NoError(t, InitFSRepoDirect(dir, 42, cfg))

	r, err := OpenFSRepo(dir, 42)
	require.NoError(t, err)

	f(r)
}

func mustGetAPIAddr(t *testing.T, r *FSRepo) string {
	rpcAddr, err := r.APIAddr()
	require.NoError(t, err)

	return rpcAddr
}

func mustSetAPIAddr(t *testing.T, r *FSRepo, addr string) {
	require.NoError(t, r.SetAPIAddr(addr))
}

func ConfigExists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "config.json"))
	if os.IsNotExist(err) {
		return false
	}
	return err == nil
}
