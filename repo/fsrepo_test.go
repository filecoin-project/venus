package repo

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	ds "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"

	"github.com/stretchr/testify/assert"

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
  accessControlAllowOrigin = ["http://localhost:8080"]

[bootstrap]
  addresses = ["TODO"]

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
