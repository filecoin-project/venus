package commands_test

import (
	"bytes"
	"encoding/json"
	"testing"

	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestDagDaemon(t *testing.T) {
	tf.IntegrationTest(t)

	t.Run("dag get <cid> returning the genesis block", func(t *testing.T) {
		d := th.NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		// get the CID of the genesis block from the "chain ls" command output

		op1 := d.RunSuccess("chain", "ls", "--enc", "json")
		result1 := op1.ReadStdoutTrimNewlines()
		genesisBlockJSONStr := bytes.Split([]byte(result1), []byte{'\n'})[0]

		var expectedRaw []types.Block
		err := json.Unmarshal(genesisBlockJSONStr, &expectedRaw)
		assert.NoError(t, err)
		require.Equal(t, 1, len(expectedRaw))
		expected := expectedRaw[0]

		// get an IPLD node from the DAG by its CID

		op2 := d.RunSuccess("dag", "get", expected.Cid().String(), "--enc", "json")

		result2 := op2.ReadStdoutTrimNewlines()

		ipldnode, err := cbor.FromJSON(bytes.NewReader([]byte(result2)), types.DefaultHashFunction, -1)
		require.NoError(t, err)

		// CBOR decode the IPLD node's raw data into a Filecoin block

		var actual types.Block
		cbor.DecodeInto(ipldnode.RawData(), &actual) // nolint: errcheck
		// assert.NoError(err)
		// TODO Enable ^^ and debug why Block.Miner isn't being de/encoded properly.

		// CIDs should be equal

		// TODO: reenable once cbor versions are matching!
		// types.AssertHaveSameCid(assert, &expected, &actual)
	})
}
