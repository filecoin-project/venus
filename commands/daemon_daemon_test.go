package commands

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	ma "gx/ipfs/QmNTCey11oxhb1AxDnQBRHtdhap6Ctud872NjAYPYYXPuc/go-multiaddr"
	manet "gx/ipfs/QmZcLBXKaFe8ND5YHPkJRAwmhJGrVsi1JqDZNyJ4nRK5Mj/go-multiaddr-net"

	th "github.com/filecoin-project/go-filecoin/testhelpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDaemonStartupMessage(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	daemon := th.NewDaemon(t).Start()
	daemon.ShutdownSuccess()

	out := daemon.ReadStdout()
	assert.Regexp("^My peer ID is [a-zA-Z0-9]*", out)
	assert.Regexp("\\nSwarm listening on.*", out)
}

func TestDaemonApiFile(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	daemon := th.NewDaemon(t).Start()

	apiPath := filepath.Join(daemon.RepoDir(), "api")
	assert.FileExists(apiPath)

	daemon.ShutdownEasy()

	_, err := os.Lstat(apiPath)
	assert.Error(err, "Expect api file to be deleted on shutdown")
	assert.True(os.IsNotExist(err))
}

func TestDaemonCORS(t *testing.T) {
	t.Parallel()
	t.Run("default allowed origins work", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		td := th.NewDaemon(t).Start()
		defer td.ShutdownSuccess()

		maddr, err := ma.NewMultiaddr(td.CmdAddr())
		assert.NoError(err)

		_, host, err := manet.DialArgs(maddr)
		assert.NoError(err)

		url := fmt.Sprintf("http://%s/api/id", host)
		req, err := http.NewRequest("GET", url, nil)
		assert.NoError(err)
		req.Header.Add("Origin", "http://localhost:8080")
		res, err := http.DefaultClient.Do(req)
		assert.NoError(err)
		assert.Equal(http.StatusOK, res.StatusCode)

		req, err = http.NewRequest("GET", url, nil)
		assert.NoError(err)
		req.Header.Add("Origin", "https://localhost:8080")
		res, err = http.DefaultClient.Do(req)
		assert.NoError(err)
		assert.Equal(http.StatusOK, res.StatusCode)

		req, err = http.NewRequest("GET", url, nil)
		assert.NoError(err)
		req.Header.Add("Origin", "http://127.0.0.1:8080")
		res, err = http.DefaultClient.Do(req)
		assert.NoError(err)
		assert.Equal(http.StatusOK, res.StatusCode)

		req, err = http.NewRequest("GET", url, nil)
		assert.NoError(err)
		req.Header.Add("Origin", "https://127.0.0.1:8080")
		res, err = http.DefaultClient.Do(req)
		assert.NoError(err)
		assert.Equal(http.StatusOK, res.StatusCode)
	})

	t.Run("non-configured origin fails", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		td := th.NewDaemon(t).Start()
		defer td.ShutdownSuccess()

		maddr, err := ma.NewMultiaddr(td.CmdAddr())
		assert.NoError(err)

		_, host, err := manet.DialArgs(maddr)
		assert.NoError(err)

		url := fmt.Sprintf("http://%s/api/id", host)
		req, err := http.NewRequest("GET", url, nil)
		assert.NoError(err)
		req.Header.Add("Origin", "http://disallowed.origin")
		res, err := http.DefaultClient.Do(req)
		assert.NoError(err)
		assert.Equal(http.StatusForbidden, res.StatusCode)
	})
}

func TestDaemonOverHttp(t *testing.T) {
	td := th.NewDaemon(t).Start()
	defer td.ShutdownSuccess()
	require := require.New(t)

	maddr, err := ma.NewMultiaddr(td.CmdAddr())
	require.NoError(err)

	_, host, err := manet.DialArgs(maddr)
	require.NoError(err)

	url := fmt.Sprintf("http://%s/api/daemon", host)
	req, err := http.NewRequest("POST", url, nil)
	require.NoError(err)
	res, err := http.DefaultClient.Do(req)
	require.NoError(err)
	require.Equal(http.StatusNotFound, res.StatusCode)
}
