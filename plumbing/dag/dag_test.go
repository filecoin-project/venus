package dag

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipfs/go-ipfs-exchange-offline"
	"github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestDAGGet(t *testing.T) {
	tf.UnitTest(t)

	t.Run("invalid ref", func(t *testing.T) {
		ctx := context.Background()

		mds := datastore.NewMapDatastore()
		bs := blockstore.NewBlockstore(mds)
		offl := offline.Exchange(bs)
		blkserv := blockservice.New(bs, offl)
		dserv := merkledag.NewDAGService(blkserv)
		dag := NewDAG(dserv)

		_, err := dag.GetNode(ctx, "awful")
		assert.EqualError(t, err, "invalid 'ipfs ref' path")
	})

	t.Run("ILPD node not found results in error", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*200)
		defer cancel()

		mds := datastore.NewMapDatastore()
		bs := blockstore.NewBlockstore(mds)
		offl := offline.Exchange(bs)
		blkserv := blockservice.New(bs, offl)
		dserv := merkledag.NewDAGService(blkserv)
		dag := NewDAG(dserv)

		someCid := types.CidFromString(t, "somecid")

		_, err := dag.GetNode(ctx, someCid.String())
		assert.EqualError(t, err, "merkledag: not found")
	})

	t.Run("matching IPLD node is emitted", func(t *testing.T) {
		ctx := context.Background()

		mds := datastore.NewMapDatastore()
		bs := blockstore.NewBlockstore(mds)
		offl := offline.Exchange(bs)
		blkserv := blockservice.New(bs, offl)
		dserv := merkledag.NewDAGService(blkserv)
		dag := NewDAG(dserv)

		ipldnode := chain.NewBuilder(t, address.Undef).NewGenesis().At(0).ToNode()

		// put into out blockservice
		assert.NoError(t, blkserv.AddBlock(ipldnode))

		res, err := dag.GetNode(ctx, ipldnode.Cid().String())
		assert.NoError(t, err)

		nodeBack, ok := res.(format.Node)
		assert.True(t, ok)
		assert.Equal(t, ipldnode.Cid().String(), nodeBack.Cid().String())
	})
}
