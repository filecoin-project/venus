package commands

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitSuccess(t *testing.T) {
	t.Run("folder exists", func(t *testing.T) {
		assert := assert.New(t)

		dir, err := ioutil.TempDir("", "init")
		assert.NoError(err)
		defer os.RemoveAll(dir)

		out, err := RunInit("--repodir", dir)
		assert.NoError(err)
		assert.Contains(string(out), fmt.Sprintf("initializing filecoin node at %s", dir))

		assert.True(ConfigExists(dir))
	})

	t.Run("folder does not exist", func(t *testing.T) {
		assert := assert.New(t)

		dir, err := ioutil.TempDir("", "init")
		assert.NoError(err)
		defer os.RemoveAll(dir)

		dir = filepath.Join(dir, "nested")

		out, err := RunInit("--repodir", dir)
		assert.NoError(err)
		assert.Contains(string(out), fmt.Sprintf("initializing filecoin node at %s", dir))

		assert.True(ConfigExists(dir))
	})
}

func TestInitFail(t *testing.T) {
	t.Run("folder is not writable", func(t *testing.T) {
		assert := assert.New(t)

		parentDir, err := ioutil.TempDir("", "init")
		assert.NoError(err)
		defer os.RemoveAll(parentDir)

		// make read only dir
		dir := filepath.Join(parentDir, "readonly")
		err = os.Mkdir(dir, 0444)
		assert.NoError(err)

		out, err := RunInit("--repodir", dir)
		assert.Error(err)
		assert.Contains(string(out), "permission denied")

		assert.False(ConfigExists(dir))
	})

	t.Run("config file already exists", func(t *testing.T) {
		assert := assert.New(t)

		dir, err := ioutil.TempDir("", "init")
		assert.NoError(err)
		defer os.RemoveAll(dir)

		ioutil.WriteFile(filepath.Join(dir, "config.toml"), []byte("hello"), 0644)
		out, err := RunInit("--repodir", dir)
		assert.Error(err)
		assert.Contains(string(out), "repo already initialized")

		assert.True(ConfigExists(dir))
	})
}
