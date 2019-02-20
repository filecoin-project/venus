package commands

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"testing"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

func TestDownloadGenesis(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port, err := th.GetFreePort()
	require.NoError(t, err)

	exec.CommandContext(
		ctx,
		th.ProjectRoot("tools/genesis-file-server/genesis-file-server"),
		"--genesis-file-path",
		th.ProjectRoot("fixtures/genesis.car"),
		"--port",
		strconv.Itoa(port),
	).Start()

	td := th.NewDaemon(t, th.GenesisFile(fmt.Sprintf("http://127.0.0.1:%d/genesis.car", port))).Start()

	td.ShutdownSuccess()
}
