package repo

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ds "gx/ipfs/QmUadX5EcvrBmxAV9sE7wUWtWSqxns5K84qKJBixmcT1w9/go-datastore"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"

	"github.com/filecoin-project/go-filecoin/config"
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
	"swarm": {
		"address": "/ip4/0.0.0.0/tcp/6000"
	},
	"mining": {
		"minerAddress": "empty",
		"autoSealIntervalSeconds": 120,
		"storagePrice": "0"
	},
	"wallet": {
		"defaultAddress": "empty"
	},
	"heartbeat": {
		"beatTarget": "",
		"beatPeriod": "3s",
		"reconnectPeriod": "10s",
		"nickname": ""
	},
	"net": ""
}`
)

func TestFSRepoInit(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	dir, err := ioutil.TempDir("", "")
	assert.NoError(err)
	defer os.RemoveAll(dir)

	t.Log("init FSRepo")
	assert.NoError(InitFSRepo(dir, config.NewDefaultConfig()))

	content, err := ioutil.ReadFile(filepath.Join(dir, configFilename))
	assert.NoError(err)

	t.Log("snapshot dir was created during FSRepo Init")
	assert.True(fileExists(filepath.Join(dir, snapshotStorePrefix)))

	// TODO: asserting the exact content here is gonna get old real quick
	t.Log("config file matches expected value")
	assert.Equal(
		expectContent,
		string(content),
	)

	version, err := ioutil.ReadFile(filepath.Join(dir, versionFilename))
	assert.NoError(err)
	assert.Equal("1", string(version))
}

func getSnapshotFilenames(t *testing.T, dir string) []string {
	require := require.New(t)

	files, err := ioutil.ReadDir(dir)
	require.NoError(err)

	var snpFiles []string
	for _, f := range files {
		if strings.Contains(f.Name(), "snapshot") {
			snpFiles = append(snpFiles, f.Name())
		}
	}
	return snpFiles
}

func TestFSRepoOpen(t *testing.T) {
	t.Parallel()
	t.Run("[fail] wrong version", func(t *testing.T) {
		assert := assert.New(t)

		dir, err := ioutil.TempDir("", "")
		assert.NoError(err)
		defer os.RemoveAll(dir)

		assert.NoError(InitFSRepo(dir, config.NewDefaultConfig()))

		// set wrong version
		assert.NoError(ioutil.WriteFile(filepath.Join(dir, versionFilename), []byte("2"), 0644))

		_, err = OpenFSRepo(dir)
		assert.EqualError(err, "invalid repo version, got 2 expected 1")
	})
}

func TestFSRepoRoundtrip(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	dir, err := ioutil.TempDir("", "")
	assert.NoError(err)
	defer os.RemoveAll(dir)

	cfg := config.NewDefaultConfig()
	cfg.API.Address = "foo" // testing that what we get back isnt just the default
	assert.NoError(err, InitFSRepo(dir, cfg))

	r, err := OpenFSRepo(dir)
	assert.NoError(err)

	assert.Equal(cfg, r.Config())
	assert.NoError(r.Datastore().Put(ds.NewKey("beep"), []byte("boop")))
	assert.NoError(r.Close())

	r2, err := OpenFSRepo(dir)
	assert.NoError(err)

	val, err := r2.Datastore().Get(ds.NewKey("beep"))
	assert.NoError(err)
	assert.Equal([]byte("boop"), val)

	assert.NoError(r2.Close())
}

func TestFSRepoReplaceAndSnapshotConfig(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)

	dir, err := ioutil.TempDir("", "")
	require.NoError(err)
	defer os.RemoveAll(dir)

	cfg := config.NewDefaultConfig()
	cfg.API.Address = "foo"
	assert.NoError(err, InitFSRepo(dir, cfg))

	expSnpsht, err := ioutil.ReadFile(filepath.Join(dir, configFilename))
	require.NoError(err)

	r1, err := OpenFSRepo(dir)
	assert.NoError(err)

	newCfg := config.NewDefaultConfig()
	newCfg.API.Address = "bar"

	assert.NoError(r1.ReplaceConfig(newCfg))
	assert.Equal("bar", r1.Config().API.Address)
	assert.NoError(r1.Close())

	r2, err := OpenFSRepo(dir)
	assert.NoError(err)
	assert.Equal("bar", r2.Config().API.Address)
	assert.NoError(r2.Close())

	// assert that a single snapshot was created when replacing the config
	// get the snapshot file name
	snpFiles := getSnapshotFilenames(t, filepath.Join(dir, snapshotStorePrefix))
	require.Equal(1, len(snpFiles))

	snpsht, err := ioutil.ReadFile(filepath.Join(dir, snapshotStorePrefix, snpFiles[0]))
	require.NoError(err)
	assert.Equal(string(expSnpsht), string(snpsht))
}

func TestRepoLock(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	dir, err := ioutil.TempDir("", "")
	assert.NoError(err)
	defer os.RemoveAll(dir)

	cfg := config.NewDefaultConfig()
	assert.NoError(err, InitFSRepo(dir, cfg))

	r, err := OpenFSRepo(dir)
	assert.NoError(err)

	assert.FileExists(filepath.Join(r.path, lockFile))

	_, err = OpenFSRepo(dir)
	assert.Error(err)
	assert.Contains(err.Error(), "failed to take repo lock")

	assert.NoError(r.Close())

	_, err = os.Lstat(filepath.Join(r.path, lockFile))
	assert.True(os.IsNotExist(err))
}

func TestRepoLockFail(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	dir, err := ioutil.TempDir("", "")
	assert.NoError(err)
	defer os.RemoveAll(dir)

	cfg := config.NewDefaultConfig()
	assert.NoError(err, InitFSRepo(dir, cfg))

	// set invalid version, to make opening the repo fail
	assert.NoError(
		ioutil.WriteFile(filepath.Join(dir, versionFilename), []byte("hello"), 0644),
	)

	_, err = OpenFSRepo(dir)
	assert.Error(err)

	_, err = os.Lstat(filepath.Join(dir, lockFile))
	assert.True(os.IsNotExist(err))
}

func TestRepoAPIFile(t *testing.T) {
	t.Parallel()
	t.Run("APIAddr returns last value written to API file", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		withFSRepo(t, func(r *FSRepo) {
			mustSetAPIAddr(t, r, "/ip4/127.0.0.1/tcp/1234")

			addr := mustGetAPIAddr(t, r)
			assert.Equal("/ip4/127.0.0.1/tcp/1234", addr)

			mustSetAPIAddr(t, r, "/ip4/127.0.0.1/tcp/4567")

			addr = mustGetAPIAddr(t, r)
			assert.Equal("/ip4/127.0.0.1/tcp/4567", addr)
		})
	})

	t.Run("SetAPIAddr is idempotent", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		withFSRepo(t, func(r *FSRepo) {
			mustSetAPIAddr(t, r, "/ip4/127.0.0.1/tcp/1234")

			addr := mustGetAPIAddr(t, r)
			assert.Equal("/ip4/127.0.0.1/tcp/1234", addr)

			mustSetAPIAddr(t, r, "/ip4/127.0.0.1/tcp/1234")
			mustSetAPIAddr(t, r, "/ip4/127.0.0.1/tcp/1234")
			mustSetAPIAddr(t, r, "/ip4/127.0.0.1/tcp/1234")
			mustSetAPIAddr(t, r, "/ip4/127.0.0.1/tcp/1234")

			addr = mustGetAPIAddr(t, r)
			assert.Equal("/ip4/127.0.0.1/tcp/1234", addr)
		})
	})

	t.Run("APIAddr fails if called before SetAPIAddr", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		withFSRepo(t, func(r *FSRepo) {
			addr, err := r.APIAddr()
			assert.Error(err)
			assert.Equal("", addr)
		})
	})

	t.Run("Close deletes API file", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		withFSRepo(t, func(r *FSRepo) {
			mustSetAPIAddr(t, r, "/ip4/127.0.0.1/tcp/1234")

			info, err := os.Stat(filepath.Join(r.path, APIFile))
			assert.NoError(err)
			assert.Equal(APIFile, info.Name())

			r.Close()

			_, err = os.Stat(filepath.Join(r.path, APIFile))
			assert.Error(err)
		})
	})

	t.Run("Close will succeed in spite of missing API file", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		withFSRepo(t, func(r *FSRepo) {
			mustSetAPIAddr(t, r, "/ip4/127.0.0.1/tcp/1234")

			err := os.Remove(filepath.Join(r.path, APIFile))
			assert.NoError(err)

			assert.NoError(r.Close())
		})
	})

	t.Run("SetAPI fails if unable to create API file", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		withFSRepo(t, func(r *FSRepo) {
			// create a file with permission bits that prevent us from truncating
			err := ioutil.WriteFile(filepath.Join(r.path, APIFile), []byte("/ip4/127.0.0.1/tcp/9999"), 0000)
			assert.NoError(err)

			// try to os.Create to same path - will see a failure
			err = r.SetAPIAddr("/ip4/127.0.0.1/tcp/1234")
			assert.Error(err)
		})
	})
}

func withFSRepo(t *testing.T, f func(*FSRepo)) {
	require := require.New(t)

	dir, err := ioutil.TempDir("", "")
	require.NoError(err)
	defer os.RemoveAll(dir)

	cfg := config.NewDefaultConfig()
	require.NoError(err, InitFSRepo(dir, cfg))

	r, err := OpenFSRepo(dir)
	require.NoError(err)

	f(r)
}

func mustGetAPIAddr(t *testing.T, r *FSRepo) string {
	require := require.New(t)

	addr, err := r.APIAddr()
	require.NoError(err)

	return addr
}

func mustSetAPIAddr(t *testing.T, r *FSRepo, addr string) {
	require := require.New(t)
	require.NoError(r.SetAPIAddr(addr))
}
