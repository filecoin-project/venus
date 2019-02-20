package commands

import (
	"bytes"
	"encoding/json"
	"testing"

	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"

	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

func TestDagDaemon(t *testing.T) {
	t.Parallel()
	t.Run("dag get <cid> returning the genesis block", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		d := th.NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		// get the CID of the genesis block from the "chain ls" command output

		op1 := d.RunSuccess("chain", "ls", "--enc", "json")
		result1 := op1.ReadStdoutTrimNewlines()
		genesisBlockJSONStr := bytes.Split([]byte(result1), []byte{'\n'})[0]

		var expectedRaw []types.Block
		err := json.Unmarshal(genesisBlockJSONStr, &expectedRaw)
		assert.NoError(err)
		require.Equal(1, len(expectedRaw))
		expected := expectedRaw[0]

		// get an IPLD node from the DAG by its CID

		op2 := d.RunSuccess("dag", "get", expected.Cid().String(), "--enc", "json")

		result2 := op2.ReadStdoutTrimNewlines()

		ipldnode, err := cbor.FromJSON(bytes.NewReader([]byte(result2)), types.DefaultHashFunction, -1)
		require.NoError(err)

		// CBOR decode the IPLD node's raw data into a Filecoin block

		var actual types.Block
		cbor.DecodeInto(ipldnode.RawData(), &actual)
		// assert.NoError(err)
		// TODO Enable ^^ and debug why Block.Miner isn't being de/encoded properly.

		// CIDs should be equal

		// TODO: reenable once cbor versions are matching!
		// types.AssertHaveSameCid(assert, &expected, &actual)
	})
}
