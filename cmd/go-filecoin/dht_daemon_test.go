package commands_test

import (
	"testing"

	ast "github.com/stretchr/testify/assert"

	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestDhtFindPeer(t *testing.T) {
	tf.IntegrationTest(t)

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
