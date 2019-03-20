package commands_test

import (
	"testing"

	ast "gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
)

func TestDhtFindPeer(t *testing.T) {
	t.Parallel()
	assert := ast.New(t)

	d1 := th.NewDaemon(t).Start()
	defer d1.ShutdownSuccess()

	d2 := th.NewDaemon(t).Start()
	defer d2.ShutdownSuccess()

	d1.ConnectSuccess(d2)

	d2Id := d2.GetID()

	findpeerOutput := d1.RunSuccess("dht", "findpeer", d2Id).ReadStdoutTrimNewlines()

	d2Addr := d2.GetAddresses()[0]

	assert.Contains(d2Addr, findpeerOutput)
}

// TODO: findprovs will have to be untested until
//  https://github.com/filecoin-project/go-filecoin/issues/2357
//  original tests were flaky; testing may need to be omitted entirely
//  unless it can consistently pass.
