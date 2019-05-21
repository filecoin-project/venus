package repo

import (
	"io/ioutil"
	"os"
	"path/filepath"
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

func TestInitRepo(t *testing.T) {
	tf.UnitTest(t)
	cfg := config.NewDefaultConfig()

	t.Run("successfully creates when directory exists", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "init")
		assert.NoError(t, err)
		defer func() {
			require.NoError(t, os.RemoveAll(dir))
		}()

		_, err = initAndOpenRepo(dir, cfg)
		assert.NoError(t, err)
		checkNewRepoFiles(t, dir)
	})

	t.Run("successfully creates when directory does not exist", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "init")
		assert.NoError(t, err)
		defer func() {
			require.NoError(t, os.RemoveAll(dir))
		}()

		dir = filepath.Join(dir, "nested")

		_, err = initAndOpenRepo(dir, cfg)
		assert.NoError(t, err)
		checkNewRepoFiles(t, dir)
	})

	t.Run("fails with error if directory is not writeable", func(t *testing.T) {
		parentDir, err := ioutil.TempDir("", "init")
		assert.NoError(t, err)
		defer func() {
			require.NoError(t, os.RemoveAll(parentDir))
		}()

		// make read only dir
		dir := filepath.Join(parentDir, "readonly")
		err = os.Mkdir(dir, 0444)
		assert.NoError(t, err)
		assert.False(t, ConfigExists(dir))

		_, err = initAndOpenRepo(dir, cfg)
		assert.Contains(t, err.Error(), "permission")
	})

	t.Run("fails with error if directory not empty", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "init")
		assert.NoError(t, err)
		defer func() {
			require.NoError(t, os.RemoveAll(dir))
		}()

		err = ioutil.WriteFile(filepath.Join(dir, "hi"), []byte("hello"), 0644)
		assert.NoError(t, err)

		_, err = initAndOpenRepo(dir, cfg)
		assert.Contains(t, err.Error(), "empty")
	})
}

func TestFSRepoOpen(t *testing.T) {
	tf.UnitTest(t)

	t.Run("[fail] repo version newer than binary", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "")
		assert.NoError(t, err)
		defer func() {
			require.NoError(t, os.RemoveAll(dir))
		}()

		assert.NoError(t, InitFSRepo(dir, config.NewDefaultConfig()))
		// set wrong version
		assert.NoError(t, ioutil.WriteFile(filepath.Join(dir, versionFilename), []byte("2"), 0644))

		_, err = OpenFSRepo(dir)
		assert.EqualError(t, err, "binary needs update to handle repo version, got 2 expected 1. Update binary to latest release")
	})
	t.Run("[fail] binary version newer than repo", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "")
		assert.NoError(t, err)
		defer func() {
			require.NoError(t, os.RemoveAll(dir))
		}()

		assert.NoError(t, InitFSRepo(dir, config.NewDefaultConfig()))
		// set wrong version
		assert.NoError(t, ioutil.WriteFile(filepath.Join(dir, versionFilename), []byte("0"), 0644))

		_, err = OpenFSRepo(dir)
		assert.EqualError(t, err, "out of date repo version, got 0 expected 1. Migrate with tools/migration/go-filecoin-migrate")
	})
	t.Run("[fail] version corrupt", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "")
		assert.NoError(t, err)
		defer func() {
			require.NoError(t, os.RemoveAll(dir))
		}()

		assert.NoError(t, InitFSRepo(dir, config.NewDefaultConfig()))
		// set wrong version
		assert.NoError(t, ioutil.WriteFile(filepath.Join(dir, versionFilename), []byte("v.8"), 0644))

		_, err = OpenFSRepo(dir)
		assert.EqualError(t, err, "failed to load version: corrupt version file: version is not an integer")
	})
}

func TestFSRepoRoundtrip(t *testing.T) {
	tf.UnitTest(t)

	dir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer func() {
		require.NoError(t, os.RemoveAll(dir))
	}()

	cfg := config.NewDefaultConfig()
	cfg.API.Address = "foo" // testing that what we get back isnt just the default

	assert.NoError(t, err, InitFSRepo(dir, cfg))

	r, err := OpenFSRepo(dir)
	assert.NoError(t, err)

	assert.Equal(t, cfg, r.Config())
	assert.NoError(t, r.Datastore().Put(ds.NewKey("beep"), []byte("boop")))
	assert.NoError(t, r.Close())

	r2, err := OpenFSRepo(dir)
	assert.NoError(t, err)

	val, err := r2.Datastore().Get(ds.NewKey("beep"))
	assert.NoError(t, err)
	assert.Equal(t, []byte("boop"), val)

	assert.NoError(t, r2.Close())
}

func TestFSRepoReplaceAndSnapshotConfig(t *testing.T) {
	tf.UnitTest(t)

	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, os.RemoveAll(dir))
	}()

	cfg := config.NewDefaultConfig()
	cfg.API.Address = "foo"
	assert.NoError(t, err, InitFSRepo(dir, cfg))

	expSnpsht, err := ioutil.ReadFile(filepath.Join(dir, configFilename))
	require.NoError(t, err)

	r1, err := OpenFSRepo(dir)
	assert.NoError(t, err)

	newCfg := config.NewDefaultConfig()
	newCfg.API.Address = "bar"

	assert.NoError(t, r1.ReplaceConfig(newCfg))
	assert.Equal(t, "bar", r1.Config().API.Address)
	assert.NoError(t, r1.Close())

	r2, err := OpenFSRepo(dir)
	assert.NoError(t, err)
	assert.Equal(t, "bar", r2.Config().API.Address)
	assert.NoError(t, r2.Close())

	// assert that a single snapshot was created when replacing the config
	// get the snapshot file name
	snpFiles := getSnapshotFilenames(t, filepath.Join(dir, snapshotStorePrefix))
	require.Equal(t, 1, len(snpFiles))

	snpsht, err := ioutil.ReadFile(filepath.Join(dir, snapshotStorePrefix, snpFiles[0]))
	require.NoError(t, err)
	assert.Equal(t, string(expSnpsht), string(snpsht))
}

func TestRepoLock(t *testing.T) {
	tf.UnitTest(t)

	dir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer func() {
		require.NoError(t, os.RemoveAll(dir))
	}()

	cfg := config.NewDefaultConfig()
	assert.NoError(t, err, InitFSRepo(dir, cfg))

	r, err := OpenFSRepo(dir)
	assert.NoError(t, err)
	assert.FileExists(t, filepath.Join(dir, lockFile))
	_, err = OpenFSRepo(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to take repo lock")
	assert.NoError(t, r.Close())

	_, err = os.Lstat(filepath.Join(dir, lockFile))
	assert.True(t, os.IsNotExist(err))
}

func TestRepoLockFail(t *testing.T) {
	tf.UnitTest(t)

	dir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer func() {
		require.NoError(t, os.RemoveAll(dir))
	}()

	cfg := config.NewDefaultConfig()
	assert.NoError(t, err, InitFSRepo(dir, cfg))

	// set invalid version, to make opening the repo fail
	assert.NoError(t,
		ioutil.WriteFile(filepath.Join(dir, versionFilename), []byte("hello"), 0644),
	)

	_, err = OpenFSRepo(dir)
	assert.Error(t, err)

	_, err = os.Lstat(filepath.Join(dir, lockFile))
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

// Inits a repo and opens it (ensuring it is openable)
func initAndOpenRepo(repoPath string, cfg *config.Config) (*FSRepo, error) {
	if err := InitFSRepo(repoPath, cfg); err != nil {
		return nil, err
	}
	return OpenFSRepo(repoPath)
}

func checkNewRepoFiles(t *testing.T, path string) {
	content, err := ioutil.ReadFile(filepath.Join(path, configFilename))
	assert.NoError(t, err)

	t.Log("snapshot path was created during FSRepo Init")
	assert.True(t, fileExists(filepath.Join(path, snapshotStorePrefix)))

	// TODO: asserting the exact content here is gonna get old real quick
	t.Log("config file matches expected value")
	assert.Equal(t, expectContent, string(content))

	version, err := ioutil.ReadFile(filepath.Join(path, versionFilename))
	assert.NoError(t, err)
	assert.Equal(t, "1", string(version))
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
	defer func() {
		require.NoError(t, os.RemoveAll(dir))
	}()

	cfg := config.NewDefaultConfig()
	require.NoError(t, err, InitFSRepo(dir, cfg))

	r, err := OpenFSRepo(dir)
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
