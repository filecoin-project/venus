package commands_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
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

		var expectedRaw []block.Block
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

		var actual block.Block
		encoding.Decode(ipldnode.RawData(), &actual) // nolint: errcheck
		// assert.NoError(err)
		// TODO Enable ^^ and debug why Block.Miner isn't being de/encoded properly.

		// CIDs should be equal

		// TODO: reenable once cbor versions are matching!
		// types.AssertHaveSameCid(assert, &expected, &actual)
	})
}
