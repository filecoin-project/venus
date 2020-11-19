package cmd_test

import (
	"bytes"
	"context"
	"testing"

	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/app/node/test"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/encoding"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func TestDagDaemon(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()

	t.Run("dag get <cid> returning the genesis block", func(t *testing.T) {
		builder := test.NewNodeBuilder(t)

		n, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		c := n.PorcelainAPI.ChainHeadKey().Iter().Value()

		// get an IPLD node from the DAG by its CID
		op := cmdClient.RunSuccess(ctx, "dag", "get", c.String(), "--enc", "json")
		result2 := op.ReadStdoutTrimNewlines()

		ipldnode, err := cbor.FromJSON(bytes.NewReader([]byte(result2)), constants.DefaultHashFunction, -1)
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
