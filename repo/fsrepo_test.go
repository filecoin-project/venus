package repo

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	ds "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/config"
)

func TestFSRepoInit(t *testing.T) {
	assert := assert.New(t)

	dir, err := ioutil.TempDir("", "")
	assert.NoError(err)
	defer os.RemoveAll(dir)

	assert.NoError(InitFSRepo(dir, config.NewDefaultConfig()))

	content, err := ioutil.ReadFile(filepath.Join(dir, configFilename))
	assert.NoError(err)

	// TODO: asserting the exact content here is gonna get old real quick
	assert.Equal(
		`[api]
  address = ":3453"
  accessControlAllowOrigin = ["http://localhost", "https://localhost", "http://127.0.0.1", "https://127.0.0.1"]
  accessControlAllowCredentials = false
  accessControlAllowMethods = ["GET", "POST", "PUT"]

[bootstrap]
  addresses = []

[datastore]
  type = "badgerds"
  path = "badger"

[swarm]
  address = "/ip4/127.0.0.1/tcp/6000"

[mining]
  rewardAddress = ""
`,
		string(content),
	)

	version, err := ioutil.ReadFile(filepath.Join(dir, versionFilename))
	assert.NoError(err)
	assert.Equal("1", string(version))
}

func TestFSRepoOpen(t *testing.T) {
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

func TestFSRepoReplaceConfig(t *testing.T) {
	assert := assert.New(t)

	dir, err := ioutil.TempDir("", "")
	assert.NoError(err)
	defer os.RemoveAll(dir)

	cfg := config.NewDefaultConfig()
	cfg.API.Address = "foo"
	assert.NoError(err, InitFSRepo(dir, cfg))

	r1, err := OpenFSRepo(dir)
	assert.NoError(err)

	newCfg := r1.Config()
	newCfg.API.Address = "bar"

	assert.NoError(r1.ReplaceConfig(newCfg))
	assert.Equal("bar", r1.Config().API.Address)
	assert.NoError(r1.Close())

	r2, err := OpenFSRepo(dir)
	assert.NoError(err)
	assert.Equal("bar", r2.Config().API.Address)
	assert.NoError(r2.Close())
}

func TestRepoLock(t *testing.T) {
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
	t.Run("APIAddr returns last value written to API file", func(t *testing.T) {
		assert := assert.New(t)

		withFSRepo(t, func(r *FSRepo) {
			mustSetAPIAddr(t, r, ":1234")

			addr := mustGetAPIAddr(t, r)
			assert.Equal(":1234", addr)

			mustSetAPIAddr(t, r, ":4567")

			addr = mustGetAPIAddr(t, r)
			assert.Equal(":4567", addr)
		})
	})

	t.Run("SetAPIAddr is idempotent", func(t *testing.T) {
		assert := assert.New(t)

		withFSRepo(t, func(r *FSRepo) {
			mustSetAPIAddr(t, r, ":1234")

			addr := mustGetAPIAddr(t, r)
			assert.Equal(":1234", addr)

			mustSetAPIAddr(t, r, ":1234")
			mustSetAPIAddr(t, r, ":1234")
			mustSetAPIAddr(t, r, ":1234")
			mustSetAPIAddr(t, r, ":1234")

			addr = mustGetAPIAddr(t, r)
			assert.Equal(":1234", addr)
		})
	})

	t.Run("APIAddr fails if called before SetAPIAddr", func(t *testing.T) {
		assert := assert.New(t)

		withFSRepo(t, func(r *FSRepo) {
			addr, err := r.APIAddr()
			assert.Error(err)
			assert.Equal("", addr)
		})
	})

	t.Run("Close deletes API file", func(t *testing.T) {
		assert := assert.New(t)

		withFSRepo(t, func(r *FSRepo) {
			mustSetAPIAddr(t, r, ":1234")

			info, err := os.Stat(filepath.Join(r.path, apiFile))
			assert.NoError(err)
			assert.Equal(apiFile, info.Name())

			r.Close()

			_, err = os.Stat(filepath.Join(r.path, apiFile))
			assert.Error(err)
		})
	})

	t.Run("Close will succeed in spite of missing API file", func(t *testing.T) {
		assert := assert.New(t)

		withFSRepo(t, func(r *FSRepo) {
			mustSetAPIAddr(t, r, ":1234")

			err := os.Remove(filepath.Join(r.path, apiFile))
			assert.NoError(err)

			assert.NoError(r.Close())
		})
	})

	t.Run("SetAPI fails if unable to create API file", func(t *testing.T) {
		assert := assert.New(t)

		withFSRepo(t, func(r *FSRepo) {
			// create a file with permission bits that prevent us from truncating
			err := ioutil.WriteFile(filepath.Join(r.path, apiFile), []byte(":9999"), 0000)
			assert.NoError(err)

			// try to os.Create to same path - will see a failure
			err = r.SetAPIAddr(":1234")
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
