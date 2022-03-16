package cmd_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	manet "github.com/multiformats/go-multiaddr/net"

	th "github.com/filecoin-project/venus/pkg/testhelpers"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadGenesis(t *testing.T) {
	tf.IntegrationTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port, err := th.GetFreePort()
	require.NoError(t, err)

	err = exec.CommandContext(
		ctx,
		th.Root("/genesis-file-server"),
		"--genesis-file-path",
		th.Root("fixtures/test/genesis.car"),
		"--port",
		strconv.Itoa(port),
	).Start()
	require.NoError(t, err)
	td := th.NewDaemon(t, th.GenesisFile(fmt.Sprintf("http://127.0.0.1:%d/genesis.car", port))).Start()

	td.ShutdownSuccess()
}

func TestDaemonStartupMessage(t *testing.T) {
	tf.IntegrationTest(t)

	daemon := th.NewDaemon(t).Start()
	daemon.ShutdownSuccess()

	out := daemon.ReadStdout()
	assert.Regexp(t, "\"My peer ID is [a-zA-Z0-9]*", out)
	assert.Regexp(t, "\\n\"Swarm listening on.*", out)
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

		maddr, err := td.CmdAddr()
		assert.NoError(t, err)

		_, host, err := manet.DialArgs(maddr) //nolint
		assert.NoError(t, err)

		url := fmt.Sprintf("http://%s/api/swarm/id", host)

		token, err := td.CmdToken()
		assert.NoError(t, err)

		req, err := http.NewRequest("POST", url, nil)
		assert.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Add("Origin", "http://localhost:8080")
		res, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.StatusCode)

		req, err = http.NewRequest("POST", url, nil)
		assert.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Add("Origin", "https://localhost:8080")
		res, err = http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.StatusCode)

		req, err = http.NewRequest("POST", url, nil)
		assert.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Add("Origin", "http://127.0.0.1:8080")
		res, err = http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.StatusCode)

		req, err = http.NewRequest("POST", url, nil)
		assert.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Add("Origin", "https://127.0.0.1:8080")
		res, err = http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.StatusCode)
	})

	t.Run("non-configured origin fails", func(t *testing.T) {
		td := th.NewDaemon(t).Start()
		defer td.ShutdownSuccess()

		maddr, err := td.CmdAddr()
		assert.NoError(t, err)

		_, host, err := manet.DialArgs(maddr) //nolint
		assert.NoError(t, err)
		token, err := td.CmdToken()
		assert.NoError(t, err)

		url := fmt.Sprintf("http://%s/api/swarm/id", host)
		req, err := http.NewRequest("POST", url, nil)
		assert.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
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

	maddr, err := td.CmdAddr()
	require.NoError(t, err)

	_, host, err := manet.DialArgs(maddr) //nolint
	require.NoError(t, err)
	token, err := td.CmdToken()
	assert.NoError(t, err)

	url := fmt.Sprintf("http://%s/api/daemon", host)
	req, err := http.NewRequest("POST", url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, res.StatusCode)
}
