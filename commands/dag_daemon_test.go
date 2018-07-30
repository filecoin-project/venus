package commands

import (
	"bytes"
	"encoding/json"
	"testing"

	cbor "gx/ipfs/QmNRz7BDWfdFNVLt7AVvmRefkrURD25EeoipcXqo6yoXU1/go-ipld-cbor"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/require"
)

func TestDagDaemon(t *testing.T) {
	t.Parallel()
	t.Run("dag get <cid> returning the genesis block", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		d := NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		// get the CID of the genesis block from the "chain ls" command output

		op1 := d.RunSuccess("chain", "ls", "--enc", "json")
		result1 := op1.readStdoutTrimNewlines()
		genesisBlockJSONStr := bytes.Split([]byte(result1), []byte{'\n'})[0]

		var expectedRaw []types.Block
		err := json.Unmarshal(genesisBlockJSONStr, &expectedRaw)
		assert.NoError(err)
		require.Equal(1, len(expectedRaw))
		expected := expectedRaw[0]

		// get an IPLD node from the DAG by its CID

		op2 := d.RunSuccess("dag", "get", expected.Cid().String(), "--enc", "json")

		result2 := op2.readStdoutTrimNewlines()

		ipldnode, err := cbor.FromJson(bytes.NewReader([]byte(result2)), types.DefaultHashFunction, -1)
		require.NoError(err)

		// CBOR decode the IPLD node's raw data into a Filecoin block

		var actual types.Block
		cbor.DecodeInto(ipldnode.RawData(), &actual)
		// assert.NoError(err)
		// TODO Enable ^^ and debug why Block.Miner isn't being de/encoded properly.

		// CIDs should be equal

		types.AssertHaveSameCid(assert, &expected, &actual)
	})
}
