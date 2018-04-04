package commands

import (
	"bytes"
	"encoding/json"
	"testing"

	cbor "gx/ipfs/QmNRz7BDWfdFNVLt7AVvmRefkrURD25EeoipcXqo6yoXU1/go-ipld-cbor"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/types"
)

func TestDagDaemon(t *testing.T) {
	t.Run("dag get <cid> returning the genesis block", func(t *testing.T) {
		assert := assert.New(t)

		d := NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		// get the CID of the genesis block from the "chain ls" command output

		op1 := d.RunSuccess("chain", "ls", "--enc", "json")
		result1 := op1.ReadStdoutTrimNewlines()

		genesisBlockJSONStr := bytes.Split([]byte(result1), []byte{'\n'})[0]

		var expected types.Block
		json.Unmarshal(genesisBlockJSONStr, &expected)

		// get an IPLD node from the DAG by its CID

		op2 := d.RunSuccess("dag", "get", expected.Cid().String(), "--enc", "json")
		result2 := op2.ReadStdoutTrimNewlines()

		ipldnode, err := cbor.FromJson(bytes.NewReader([]byte(result2)), types.DefaultHashFunction, -1)
		assert.NoError(err)

		// CBOR decode the IPLD node's raw data into a Filecoin block

		var actual types.Block
		cbor.DecodeInto(ipldnode.RawData(), &actual)

		// CIDs should be equal

		types.AssertHaveSameCid(assert, &expected, &actual)
	})
}
