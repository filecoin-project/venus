package repo

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/filecoin-project/go-filecoin/config"
	"github.com/stretchr/testify/assert"
	ds "gx/ipfs/QmPpegoMqhAEqjncrzArm7KVWAkCm78rqL2DPuNjhPrshg/go-datastore"
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

[bootstrap]
  addresses = ["TODO"]

[datastore]
  type = "badgerds"
  path = "badger"

[swarm]
  address = "/ip4/127.0.0.1/tcp/6000"
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
