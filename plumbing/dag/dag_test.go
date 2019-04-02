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

	"github.com/filecoin-project/go-filecoin/types"
)

func TestDAGGet(t *testing.T) {
	t.Parallel()

	t.Run("invalid ref", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		ctx := context.Background()

		mds := datastore.NewMapDatastore()
		bs := blockstore.NewBlockstore(mds)
		offl := offline.Exchange(bs)
		blkserv := blockservice.New(bs, offl)
		dserv := merkledag.NewDAGService(blkserv)
		dag := NewDAG(dserv)

		_, err := dag.GetNode(ctx, "awful")
		assert.EqualError(err, "invalid 'ipfs ref' path")
	})

	t.Run("ILPD node not found results in error", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*200)
		defer cancel()

		mds := datastore.NewMapDatastore()
		bs := blockstore.NewBlockstore(mds)
		offl := offline.Exchange(bs)
		blkserv := blockservice.New(bs, offl)
		dserv := merkledag.NewDAGService(blkserv)
		dag := NewDAG(dserv)

		someCid := types.SomeCid()

		_, err := dag.GetNode(ctx, someCid.String())
		assert.EqualError(err, "merkledag: not found")
	})

	t.Run("matching IPLD node is emitted", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		ctx := context.Background()

		mds := datastore.NewMapDatastore()
		bs := blockstore.NewBlockstore(mds)
		offl := offline.Exchange(bs)
		blkserv := blockservice.New(bs, offl)
		dserv := merkledag.NewDAGService(blkserv)
		dag := NewDAG(dserv)

		ipldnode := types.NewBlockForTest(nil, 1234).ToNode()

		// put into out blockservice
		assert.NoError(blkserv.AddBlock(ipldnode))

		res, err := dag.GetNode(ctx, ipldnode.Cid().String())
		assert.NoError(err)

		nodeBack, ok := res.(format.Node)
		assert.True(ok)
		assert.Equal(ipldnode.Cid().String(), nodeBack.Cid().String())
	})
}
