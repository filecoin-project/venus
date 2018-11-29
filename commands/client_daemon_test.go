package commands

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/fixtures"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
)

func TestListAsks(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	peer := th.NewDaemon(t, th.WithMiner(fixtures.TestMiners[0]), th.KeyFile(fixtures.KeyFilePaths()[2])).Start()
	defer peer.ShutdownSuccess()
	d := th.NewDaemon(t, th.KeyFile(fixtures.KeyFilePaths()[2])).Start()
	defer d.ShutdownSuccess()
	peer.ConnectSuccess(d)

	// create a miner with one ask
	minerAddr := d.CreateMinerAddr(peer, fixtures.TestAddresses[2])
	d.CreateAsk(peer, minerAddr.String(), fixtures.TestAddresses[2], "20", "10")

	// check ls
	expectedBaseHeight := 2
	expectedExpiry := expectedBaseHeight + 10
	ls := d.RunSuccess("client", "list-asks")
	expectedResult := fmt.Sprintf("%s 000 20 %d", minerAddr.String(), expectedExpiry)
	assert.Equal(expectedResult, strings.Trim(ls.ReadStdout(), "\n"))

}
