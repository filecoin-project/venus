package commands

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	manet "gx/ipfs/QmV6FjemM1K8oXjrvuq3wuVWWoU2TLDPmNnKrxHzY3v6Ai/go-multiaddr-net"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/stretchr/testify/assert"
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

		maddr, err := ma.NewMultiaddr(td.CmdAddr())
		assert.NoError(err)

		_, host, err := manet.DialArgs(maddr)
		assert.NoError(err)

		url := fmt.Sprintf("http://%s/api/id", host)
		req, err := http.NewRequest("GET", url, nil)
		assert.NoError(err)
		req.Header.Add("Origin", "http://localhost")
		res, err := http.DefaultClient.Do(req)
		assert.NoError(err)
		assert.Equal(http.StatusOK, res.StatusCode)

		req, err = http.NewRequest("GET", url, nil)
		assert.NoError(err)
		req.Header.Add("Origin", "https://localhost")
		res, err = http.DefaultClient.Do(req)
		assert.NoError(err)
		assert.Equal(http.StatusOK, res.StatusCode)

		req, err = http.NewRequest("GET", url, nil)
		assert.NoError(err)
		req.Header.Add("Origin", "http://127.0.0.1")
		res, err = http.DefaultClient.Do(req)
		assert.NoError(err)
		assert.Equal(http.StatusOK, res.StatusCode)

		req, err = http.NewRequest("GET", url, nil)
		assert.NoError(err)
		req.Header.Add("Origin", "https://127.0.0.1")
		res, err = http.DefaultClient.Do(req)
		assert.NoError(err)
		assert.Equal(http.StatusOK, res.StatusCode)
	})

	t.Run("non-configured origin fails", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		td := th.NewDaemon(t).Start()

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
