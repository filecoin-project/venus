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

	assert := assert.New(t)
	daemon := th.NewDaemon(t).Start()
	daemon.ShutdownSuccess()

	out := daemon.ReadStdout()
	assert.Regexp("^My peer ID is [a-zA-Z0-9]*", out)
	assert.Regexp("\\nSwarm listening on.*", out)
}

func TestDaemonApiFile(t *testing.T) {
	tf.IntegrationTest(t)

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
	tf.IntegrationTest(t)

	t.Run("default allowed origins work", func(t *testing.T) {

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
	tf.IntegrationTest(t)

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
