package repo

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/filecoin-project/go-filecoin/config"
	"github.com/stretchr/testify/assert"
)

func TestFSRepoInit(t *testing.T) {
	assert := assert.New(t)

	dir, err := ioutil.TempDir("", "")
	assert.NoError(err)
	defer os.RemoveAll(dir)

	assert.NoError(InitFSRepo(dir, config.NewDefaultConfig()))

	content, err := ioutil.ReadFile(filepath.Join(dir, configFilename))
	assert.NoError(err)

	assert.Equal(
		`[api]
  address = ":3453"

[swarm]
  address = "/ip4/127.0.0.1/tcp/6000"

[bootstrap]
  addresses = ["TODO"]
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
