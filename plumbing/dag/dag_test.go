package dag

import (
	"context"
	"testing"
	"time"

	"gx/ipfs/QmNRAuGmvnVw8urHkUZQirhu42VTiZjVWASa2aTznEMmpP/go-merkledag"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmRL22E4paat7ky7vx9MLpR97JHHbFPrg3ytFQw6qp1y1s/go-ipld-format"
	"gx/ipfs/QmRu7tiRnFk9mMPpVECQTBQJqXtmG132jJxA1w9A7TtpBz/go-ipfs-blockstore"
	"gx/ipfs/QmSz8kAe2JCKp2dWSG8gHSWnwSmne8YfRXTeK5HBmc9L7t/go-ipfs-exchange-offline"
	"gx/ipfs/QmUadX5EcvrBmxAV9sE7wUWtWSqxns5K84qKJBixmcT1w9/go-datastore"
	"gx/ipfs/QmZsGVGCqMCNzHLNMB6q4F6yyvomqf1VxwhJwSfgo1NGaF/go-blockservice"

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

		_, err := dag.Get(ctx, "awful")
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

		_, err := dag.Get(ctx, someCid.String())
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

		res, err := dag.Get(ctx, ipldnode.Cid().String())
		assert.NoError(err)

		nodeBack, ok := res.(format.Node)
		assert.True(ok)
		assert.Equal(ipldnode.Cid().String(), nodeBack.Cid().String())
	})
}
