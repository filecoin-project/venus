package commands_test

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"testing"

	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"

	"github.com/stretchr/testify/require"
)

func TestInitOverHttp(t *testing.T) {
	tf.IntegrationTest(t)

	td := th.NewDaemon(t).Start()
	defer td.ShutdownSuccess()
	require := require.New(t)

	maddr, err := ma.NewMultiaddr(td.CmdAddr())
	require.NoError(err)

	_, host, err := manet.DialArgs(maddr)
	require.NoError(err)

	url := fmt.Sprintf("http://%s/api/init", host)
	req, err := http.NewRequest("POST", url, nil)
	require.NoError(err)
	res, err := http.DefaultClient.Do(req)
	require.NoError(err)
	require.Equal(http.StatusNotFound, res.StatusCode)
}

func TestDownloadGenesis(t *testing.T) {
	tf.IntegrationTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port, err := th.GetFreePort()
	require.NoError(t, err)

	err = exec.CommandContext(
		ctx,
		th.ProjectRoot("tools/genesis-file-server/genesis-file-server"),
		"--genesis-file-path",
		th.ProjectRoot("fixtures/genesis.car"),
		"--port",
		strconv.Itoa(port),
	).Start()
	require.NoError(t, err)

	td := th.NewDaemon(t, th.GenesisFile(fmt.Sprintf("http://127.0.0.1:%d/genesis.car", port))).Start()

	td.ShutdownSuccess()
}
