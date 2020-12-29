package cmd_test

import (
	"fmt"
	"net/http"
	"testing"

	manet "github.com/multiformats/go-multiaddr-net" //nolint

	"github.com/stretchr/testify/require"

	th "github.com/filecoin-project/venus/pkg/testhelpers"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func TestInitOverHttp(t *testing.T) {
	tf.IntegrationTest(t)

	td := th.NewDaemon(t).Start()
	defer td.ShutdownSuccess()

	maddr, err := td.CmdAddr()
	require.NoError(t, err)

	_, host, err := manet.DialArgs(maddr) //nolint
	require.NoError(t, err)

	url := fmt.Sprintf("http://%s/api/init", host)
	req, err := http.NewRequest("POST", url, nil)
	require.NoError(t, err)
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, res.StatusCode)
}
