package impl

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/filecoin-project/go-filecoin/api"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
)

func TestDaemonInitSuccess(t *testing.T) {
	t.Run("folder exists", func(t *testing.T) {
		assert := assert.New(t)
		ctx := context.Background()
		dir, err := ioutil.TempDir("", "init")
		assert.NoError(err)
		defer os.RemoveAll(dir)

		// TODO: pass cmdapi flag to RunInit to allow for parallel test running
		err = New(nil).Daemon().Init(ctx, api.RepoDir(dir))
		assert.NoError(err)

		assert.True(ConfigExists(dir))
	})

	t.Run("folder does not exist", func(t *testing.T) {
		assert := assert.New(t)
		ctx := context.Background()

		dir, err := ioutil.TempDir("", "init")
		assert.NoError(err)
		defer os.RemoveAll(dir)

		dir = filepath.Join(dir, "nested")

		err = New(nil).Daemon().Init(ctx, api.RepoDir(dir))
		assert.NoError(err)

		assert.True(ConfigExists(dir))
	})
}

func TestDaemonInitFail(t *testing.T) {
	t.Run("folder is not writable", func(t *testing.T) {
		assert := assert.New(t)
		ctx := context.Background()
		parentDir, err := ioutil.TempDir("", "init")
		assert.NoError(err)
		defer os.RemoveAll(parentDir)

		// make read only dir
		dir := filepath.Join(parentDir, "readonly")
		err = os.Mkdir(dir, 0444)
		assert.NoError(err)

		err = New(nil).Daemon().Init(ctx, api.RepoDir(dir))
		assert.Contains(err.Error(), "permission denied")

		assert.False(ConfigExists(dir))
	})

	t.Run("config file already exists", func(t *testing.T) {
		assert := assert.New(t)
		ctx := context.Background()

		dir, err := ioutil.TempDir("", "init")
		assert.NoError(err)
		defer os.RemoveAll(dir)

		ioutil.WriteFile(filepath.Join(dir, "config.json"), []byte("hello"), 0644)

		err = New(nil).Daemon().Init(ctx, api.RepoDir(dir))
		assert.Contains(err.Error(), "repo already initialized")

		assert.True(ConfigExists(dir))
	})
}

func ConfigExists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "config.json"))
	if os.IsNotExist(err) {
		return false
	}
	return err == nil
}
