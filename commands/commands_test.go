package commands_test

import (
	"fmt"
	"os"
	"testing"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"

	"github.com/filecoin-project/go-filecoin/fixtures"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/xeipuuv/gojsonschema"
)

func requireSchemaConformance(t *testing.T, jsonBytes []byte, schemaName string) { // nolint: deadcode
	wdir, _ := os.Getwd()
	rLoader := gojsonschema.NewReferenceLoader(fmt.Sprintf("file://%s/schema/%s.schema.json", wdir, schemaName))
	jLoader := gojsonschema.NewBytesLoader(jsonBytes)

	result, err := gojsonschema.Validate(rLoader, jLoader)
	require.NoError(t, err)

	for _, desc := range result.Errors() {
		t.Errorf("- %s\n", desc)
	}

	require.Truef(t, result.Valid(), "Error schema validating: %s", string(jsonBytes))
}

// create a basic new TestDaemon, with a miner and the KeyInfo it needs to sign
// tickets and blocks. This does not set a DefaultAddress in the Wallet; in this
// case, node/init.go Init generates a new address in the wallet and sets it to
// the default address.
func makeTestDaemonWithMinerAndStart(t *testing.T) *th.TestDaemon {
	daemon := th.NewDaemon(
		t,
		th.WithMiner(fixtures.TestMiners[0]),
		th.KeyFile(fixtures.KeyFilePaths()[0]),
	).Start()
	return daemon
}
