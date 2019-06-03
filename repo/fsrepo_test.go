package repo

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	ds "github.com/ipfs/go-datastore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/config"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

const (
	expectContent = `{
	"api": {
		"address": "/ip4/127.0.0.1/tcp/3453",
		"accessControlAllowOrigin": [
			"http://localhost:8080",
			"https://localhost:8080",
			"http://127.0.0.1:8080",
			"https://127.0.0.1:8080"
		],
		"accessControlAllowCredentials": false,
		"accessControlAllowMethods": [
			"GET",
			"POST",
			"PUT"
		]
	},
	"bootstrap": {
		"addresses": [],
		"minPeerThreshold": 0,
		"period": "1m"
	},
	"datastore": {
		"type": "badgerds",
		"path": "badger"
	},
	"heartbeat": {
		"beatTarget": "",
		"beatPeriod": "3s",
		"reconnectPeriod": "10s",
		"nickname": ""
	},
	"mining": {
		"minerAddress": "empty",
		"autoSealIntervalSeconds": 120,
		"storagePrice": "0"
	},
	"mpool": {
		"maxPoolSize": 10000,
		"maxNonceGap": "100"
	},
	"net": "",
	"observability": {
		"metrics": {
			"prometheusEnabled": false,
			"reportInterval": "5s",
			"prometheusEndpoint": "/ip4/0.0.0.0/tcp/9400"
		},
		"tracing": {
			"jaegerTracingEnabled": false,
			"probabilitySampler": 1,
			"jaegerEndpoint": "http://localhost:14268/api/traces"
		}
	},
	"sectorbase": {
		"rootdir": ""
	},
	"swarm": {
		"address": "/ip4/0.0.0.0/tcp/6000"
	},
	"wallet": {
		"defaultAddress": "empty"
	}
}`
)

func TestInitRepoDirect(t *testing.T) {
	tf.UnitTest(t)
	cfg := config.NewDefaultConfig()

	// Inits a repo and opens it (ensuring it is openable)
	initAndOpenRepoDirect := func(repoPath string, version uint, cfg *config.Config) (*FSRepo, error) {
		if err := InitFSRepoDirect(repoPath, version, cfg); err != nil {
			return nil, err
		}
		return OpenFSRepo(repoPath, version)
	}

	t.Run("successfully creates when directory exists", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "init")
		require.NoError(t, err)
		defer RequireRemoveAll(t, dir)

		_, err = initAndOpenRepoDirect(dir, 42, cfg)
		assert.NoError(t, err)
		checkNewRepoFiles(t, dir, 42)
	})

	t.Run("successfully creates when directory does not exist", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "init")
		require.NoError(t, err)
		defer func() {
			require.NoError(t, os.RemoveAll(dir))
		}()

		dir = filepath.Join(dir, "nested")

		_, err = initAndOpenRepoDirect(dir, 42, cfg)
		assert.NoError(t, err)
		checkNewRepoFiles(t, dir, 42)
	})

	t.Run("fails with error if directory is not writeable", func(t *testing.T) {
		parentDir, err := ioutil.TempDir("", "init")
		require.NoError(t, err)
		defer RequireRemoveAll(t, parentDir)

		// make read only dir
		dir := filepath.Join(parentDir, "readonly")
		err = os.Mkdir(dir, 0444)
		assert.NoError(t, err)
		assert.False(t, ConfigExists(dir))

		_, err = initAndOpenRepoDirect(dir, 42, cfg)
		assert.Contains(t, err.Error(), "permission")
	})

	t.Run("fails with error if directory not empty", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "init")
		require.NoError(t, err)
		defer RequireRemoveAll(t, dir)

		err = ioutil.WriteFile(filepath.Join(dir, "hi"), []byte("hello"), 0644)
		assert.NoError(t, err)

		_, err = initAndOpenRepoDirect(dir, 42, cfg)
		assert.Contains(t, err.Error(), "empty")
	})
}

func TestFSRepoOpen(t *testing.T) {
	tf.UnitTest(t)

	t.Run("[fail] repo version newer than binary", func(t *testing.T) {
		container, err := ioutil.TempDir("", "")
		require.NoError(t, err)
		defer RequireRemoveAll(t, container)
		repoPath := path.Join(container, "repo")

		assert.NoError(t, InitFSRepo(repoPath, 1, config.NewDefaultConfig()))
		// set wrong version
		assert.NoError(t, WriteVersion(repoPath, 99))

		_, err = OpenFSRepo(repoPath, 1)
		expected := fmt.Sprintf("binary needs update to handle repo version, got 99 expected %d. Update binary to latest release", Version)
		assert.EqualError(t, err, expected)
	})
	t.Run("[fail] binary version newer than repo", func(t *testing.T) {
		container, err := ioutil.TempDir("", "")
		require.NoError(t, err)
		defer RequireRemoveAll(t, container)
		repoPath := path.Join(container, "repo")

		assert.NoError(t, InitFSRepo(repoPath, 1, config.NewDefaultConfig()))
		// set wrong version
		assert.NoError(t, WriteVersion(repoPath, 0))

		_, err = OpenFSRepo(repoPath, 1)
		expected := fmt.Sprintf("out of date repo version, got 0 expected %d. Migrate with tools/migration/go-filecoin-migrate", Version)

		assert.EqualError(t, err, expected)
	})
	t.Run("[fail] version corrupt", func(t *testing.T) {
		container, err := ioutil.TempDir("", "")
		require.NoError(t, err)
		defer RequireRemoveAll(t, container)
		repoPath := path.Join(container, "repo")

		assert.NoError(t, InitFSRepo(repoPath, 1, config.NewDefaultConfig()))
		// set wrong version
		assert.NoError(t, ioutil.WriteFile(filepath.Join(repoPath, versionFilename), []byte("v.8"), 0644))

		_, err = OpenFSRepo(repoPath, 1)
		assert.EqualError(t, err, "failed to read version: corrupt version file: version is not an integer")
	})
}

func TestFSRepoRoundtrip(t *testing.T) {
	tf.UnitTest(t)

	container, err := ioutil.TempDir("", "container")
	require.NoError(t, err)
	defer RequireRemoveAll(t, container)

	cfg := config.NewDefaultConfig()
	cfg.API.Address = "foo" // testing that what we get back isnt just the default

	repoPath := path.Join(container, "repo")
	assert.NoError(t, err, InitFSRepo(repoPath, 42, cfg))

	r, err := OpenFSRepo(repoPath, 42)
	assert.NoError(t, err)

	assert.Equal(t, cfg, r.Config())
	assert.NoError(t, r.Datastore().Put(ds.NewKey("beep"), []byte("boop")))
	assert.NoError(t, r.Close())

	r2, err := OpenFSRepo(repoPath, 42)
	assert.NoError(t, err)

	val, err := r2.Datastore().Get(ds.NewKey("beep"))
	assert.NoError(t, err)
	assert.Equal(t, []byte("boop"), val)

	assert.NoError(t, r2.Close())
}

func TestFSRepoReplaceAndSnapshotConfig(t *testing.T) {
	tf.UnitTest(t)

	container, err := ioutil.TempDir("", "container")
	require.NoError(t, err)
	defer RequireRemoveAll(t, container)
	repoPath := path.Join(container, "repo")

	cfg := config.NewDefaultConfig()
	cfg.API.Address = "foo"
	assert.NoError(t, err, InitFSRepo(repoPath, 42, cfg))

	expSnpsht, err := ioutil.ReadFile(filepath.Join(repoPath, configFilename))
	require.NoError(t, err)

	r1, err := OpenFSRepo(repoPath, 42)
	assert.NoError(t, err)

	newCfg := config.NewDefaultConfig()
	newCfg.API.Address = "bar"

	assert.NoError(t, r1.ReplaceConfig(newCfg))
	assert.Equal(t, "bar", r1.Config().API.Address)
	assert.NoError(t, r1.Close())

	r2, err := OpenFSRepo(repoPath, 42)
	assert.NoError(t, err)
	assert.Equal(t, "bar", r2.Config().API.Address)
	assert.NoError(t, r2.Close())

	// assert that a single snapshot was created when replacing the config
	// get the snapshot file name
	snpFiles := getSnapshotFilenames(t, filepath.Join(repoPath, snapshotStorePrefix))
	require.Equal(t, 1, len(snpFiles))

	snpsht, err := ioutil.ReadFile(filepath.Join(repoPath, snapshotStorePrefix, snpFiles[0]))
	require.NoError(t, err)
	assert.Equal(t, string(expSnpsht), string(snpsht))
}

func TestRepoLock(t *testing.T) {
	tf.UnitTest(t)

	container, err := ioutil.TempDir("", "container")
	require.NoError(t, err)
	defer RequireRemoveAll(t, container)
	repoPath := path.Join(container, "repo")

	cfg := config.NewDefaultConfig()
	assert.NoError(t, err, InitFSRepo(repoPath, 42, cfg))

	r, err := OpenFSRepo(repoPath, 42)
	assert.NoError(t, err)
	assert.FileExists(t, filepath.Join(repoPath, lockFile))
	_, err = OpenFSRepo(repoPath, 42)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to take repo lock")
	assert.NoError(t, r.Close())

	_, err = os.Lstat(filepath.Join(repoPath, lockFile))
	assert.True(t, os.IsNotExist(err))
}

func TestRepoLockFail(t *testing.T) {
	tf.UnitTest(t)

	container, err := ioutil.TempDir("", "container")
	require.NoError(t, err)
	defer RequireRemoveAll(t, container)
	repoPath := path.Join(container, "repo")

	cfg := config.NewDefaultConfig()
	assert.NoError(t, err, InitFSRepo(repoPath, 42, cfg))

	// set invalid version, to make opening the repo fail
	assert.NoError(t,
		ioutil.WriteFile(filepath.Join(repoPath, versionFilename), []byte("hello"), 0644),
	)

	_, err = OpenFSRepo(repoPath, 42)
	assert.Error(t, err)

	_, err = os.Lstat(filepath.Join(repoPath, lockFile))
	assert.True(t, os.IsNotExist(err))
}

func TestRepoAPIFile(t *testing.T) {
	tf.UnitTest(t)

	t.Run("APIAddr returns last value written to API file", func(t *testing.T) {
		withFSRepo(t, func(r *FSRepo) {
			mustSetAPIAddr(t, r, "/ip4/127.0.0.1/tcp/1234")

			addr := mustGetAPIAddr(t, r)
			assert.Equal(t, "/ip4/127.0.0.1/tcp/1234", addr)

			mustSetAPIAddr(t, r, "/ip4/127.0.0.1/tcp/4567")

			addr = mustGetAPIAddr(t, r)
			assert.Equal(t, "/ip4/127.0.0.1/tcp/4567", addr)
		})
	})

	t.Run("SetAPIAddr is idempotent", func(t *testing.T) {
		withFSRepo(t, func(r *FSRepo) {
			mustSetAPIAddr(t, r, "/ip4/127.0.0.1/tcp/1234")

			addr := mustGetAPIAddr(t, r)
			assert.Equal(t, "/ip4/127.0.0.1/tcp/1234", addr)

			mustSetAPIAddr(t, r, "/ip4/127.0.0.1/tcp/1234")
			mustSetAPIAddr(t, r, "/ip4/127.0.0.1/tcp/1234")
			mustSetAPIAddr(t, r, "/ip4/127.0.0.1/tcp/1234")
			mustSetAPIAddr(t, r, "/ip4/127.0.0.1/tcp/1234")

			addr = mustGetAPIAddr(t, r)
			assert.Equal(t, "/ip4/127.0.0.1/tcp/1234", addr)
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
			err := ioutil.WriteFile(filepath.Join(r.path, apiFile), []byte("/ip4/127.0.0.1/tcp/9999"), 0000)
			assert.NoError(t, err)

			// try to os.Create to same path - will see a failure
			err = r.SetAPIAddr("/ip4/127.0.0.1/tcp/1234")
			assert.Error(t, err)
		})
	})
}

func checkNewRepoFiles(t *testing.T, path string, version uint) {
	content, err := ioutil.ReadFile(filepath.Join(path, configFilename))
	assert.NoError(t, err)

	t.Log("snapshot path was created during FSRepo Init")
	exists, err := fileExists(filepath.Join(path, snapshotStorePrefix))
	assert.NoError(t, err)
	assert.True(t, exists)

	// Asserting the exact content here is gonna get old real quick
	t.Log("config file matches expected value")
	assert.Equal(t, expectContent, string(content))

	actualVersion, err := ioutil.ReadFile(filepath.Join(path, versionFilename))
	assert.NoError(t, err)
	assert.Equal(t, strconv.FormatUint(uint64(version), 10), string(actualVersion))
}

func getSnapshotFilenames(t *testing.T, dir string) []string {
	files, err := ioutil.ReadDir(dir)
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
	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer RequireRemoveAll(t, dir)

	cfg := config.NewDefaultConfig()
	require.NoError(t, err, InitFSRepoDirect(dir, 42, cfg))

	r, err := OpenFSRepo(dir, 42)
	require.NoError(t, err)

	f(r)
}

func mustGetAPIAddr(t *testing.T, r *FSRepo) string {
	addr, err := r.APIAddr()
	require.NoError(t, err)

	return addr
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
