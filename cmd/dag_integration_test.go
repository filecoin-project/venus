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
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func TestDagDaemon(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()

	t.Run("dag get <cid> returning the genesis block", func(t *testing.T) {
		builder := test.NewNodeBuilder(t)

		n, cmdClient, done := builder.BuildAndStartAPI(ctx)
		defer done()

		head, err := n.Chain().API().ChainHead(context.Background())
		require.NoError(t, err)
		hb := head.Key().Cids()[0]
		// get an IPLD node from the DAG by its CID
		op := cmdClient.RunSuccess(ctx, "dag", "get", hb.String(), "--enc", "json")
		result2 := op.ReadStdoutTrimNewlines()

		ipldnode, err := cbor.FromJSON(bytes.NewReader([]byte(result2)), constants.DefaultHashFunction, -1)
		require.NoError(t, err)

		// CBOR decode the IPLD node's raw data into a Filecoin block

		var actual block.Block
		actual.UnmarshalCBOR(bytes.NewReader(ipldnode.RawData())) // nolint: errcheck
		// assert.NoError(err)
		// TODO Enable ^^ and debug why Block.Miner isn't being de/encoded properly.

		// CIDs should be equal

		// TODO: reenable once cbor versions are matching!
		// types.AssertHaveSameCid(assert, &expected, &actual)
	})
}
