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
		string(content),
		`[api]
  address = ":3453"

[swarm]
  address = "/ip4/127.0.0.1/tcp/6000"
`,
	)

}
