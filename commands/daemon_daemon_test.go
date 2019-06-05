package commands_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

func TestDaemonStartupMessage(t *testing.T) {
	tf.IntegrationTest(t)

	daemon := th.NewDaemon(t).Start()
	daemon.ShutdownSuccess()

	out := daemon.ReadStdout()
	assert.Regexp(t, "^My peer ID is [a-zA-Z0-9]*", out)
	assert.Regexp(t, "\\nSwarm listening on.*", out)
}

func TestDaemonApiFile(t *testing.T) {
	tf.IntegrationTest(t)

	daemon := th.NewDaemon(t).Start()

	apiPath := filepath.Join(daemon.RepoDir(), "api")
	assert.FileExists(t, apiPath)

	daemon.ShutdownEasy()

	_, err := os.Lstat(apiPath)
	assert.Error(t, err, "Expect api file to be deleted on shutdown")
	assert.True(t, os.IsNotExist(err))
}

func TestDaemonCORS(t *testing.T) {
	tf.IntegrationTest(t)

	t.Run("default allowed origins work", func(t *testing.T) {
		td := th.NewDaemon(t).Start()
		defer td.ShutdownSuccess()

		maddr, err := ma.NewMultiaddr(td.CmdAddr())
		assert.NoError(t, err)

		_, host, err := manet.DialArgs(maddr)
		assert.NoError(t, err)

		url := fmt.Sprintf("http://%s/api/id", host)
		req, err := http.NewRequest("GET", url, nil)
		assert.NoError(t, err)
		req.Header.Add("Origin", "http://localhost:8080")
		res, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.StatusCode)

		req, err = http.NewRequest("GET", url, nil)
		assert.NoError(t, err)
		req.Header.Add("Origin", "https://localhost:8080")
		res, err = http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.StatusCode)

		req, err = http.NewRequest("GET", url, nil)
		assert.NoError(t, err)
		req.Header.Add("Origin", "http://127.0.0.1:8080")
		res, err = http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.StatusCode)

		req, err = http.NewRequest("GET", url, nil)
		assert.NoError(t, err)
		req.Header.Add("Origin", "https://127.0.0.1:8080")
		res, err = http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.StatusCode)
	})

	t.Run("non-configured origin fails", func(t *testing.T) {
		td := th.NewDaemon(t).Start()
		defer td.ShutdownSuccess()

		maddr, err := ma.NewMultiaddr(td.CmdAddr())
		assert.NoError(t, err)

		_, host, err := manet.DialArgs(maddr)
		assert.NoError(t, err)

		url := fmt.Sprintf("http://%s/api/id", host)
		req, err := http.NewRequest("GET", url, nil)
		assert.NoError(t, err)
		req.Header.Add("Origin", "http://disallowed.origin")
		res, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, res.StatusCode)
	})
}

func TestDaemonOverHttp(t *testing.T) {
	tf.IntegrationTest(t)

	td := th.NewDaemon(t).Start()
	defer td.ShutdownSuccess()

	maddr, err := ma.NewMultiaddr(td.CmdAddr())
	require.NoError(t, err)

	_, host, err := manet.DialArgs(maddr)
	require.NoError(t, err)

	url := fmt.Sprintf("http://%s/api/daemon", host)
	req, err := http.NewRequest("POST", url, nil)
	require.NoError(t, err)
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, res.StatusCode)
}
