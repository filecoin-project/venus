package commands_test

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"testing"

	manet "github.com/multiformats/go-multiaddr-net"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/build/project"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestInitOverHttp(t *testing.T) {
	tf.IntegrationTest(t)

	td := th.NewDaemon(t).Start()
	defer td.ShutdownSuccess()

	maddr, err := td.CmdAddr()
	require.NoError(t, err)

	_, host, err := manet.DialArgs(maddr)
	require.NoError(t, err)

	url := fmt.Sprintf("http://%s/api/init", host)
	req, err := http.NewRequest("POST", url, nil)
	require.NoError(t, err)
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, res.StatusCode)
}

func TestDownloadGenesis(t *testing.T) {
	tf.IntegrationTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port, err := th.GetFreePort()
	require.NoError(t, err)

	err = exec.CommandContext(
		ctx,
		project.Root("tools/genesis-file-server/genesis-file-server"),
		"--genesis-file-path",
		project.Root("fixtures/test/genesis.car"),
		"--port",
		strconv.Itoa(port),
	).Start()
	require.NoError(t, err)

	td := th.NewDaemon(t, th.GenesisFile(fmt.Sprintf("http://127.0.0.1:%d/genesis.car", port))).Start()

	td.ShutdownSuccess()
}
