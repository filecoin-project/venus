package commands

import (
	"fmt"
	"net/http"
	"testing"

	manet "gx/ipfs/QmQVUtnrNGtCRkCMpXgpApfzQjc8FDaDVxHqWH8cnZQeh5/go-multiaddr-net"
	ma "gx/ipfs/QmRKLtwMw131aK7ugC3G7ybpumMz78YrJe5dzneyindvG1/go-multiaddr"

	th "github.com/filecoin-project/go-filecoin/testhelpers"

	"github.com/stretchr/testify/require"
)

func TestInitOverHttp(t *testing.T) {
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
