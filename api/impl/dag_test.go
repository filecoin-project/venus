package impl

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gx/ipfs/QmRL22E4paat7ky7vx9MLpR97JHHbFPrg3ytFQw6qp1y1s/go-ipld-format"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"

	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestDagGet(t *testing.T) {
	t.Parallel()

	t.Run("invalid ref", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		ctx := context.Background()
		n := node.MakeOfflineNode(t)
		api := New(n)

		_, err := api.Dag().Get(ctx, "awful")
		assert.EqualError(err, "invalid 'ipfs ref' path")
	})

	t.Run("ILPD node not found results in error", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*200)
		defer cancel()
		n := node.MakeOfflineNode(t)
		api := New(n)

		someCid := types.SomeCid()

		_, err := api.Dag().Get(ctx, someCid.String())
		assert.EqualError(err, fmt.Sprintf("failed to get block for %s: context deadline exceeded", someCid.String()))
	})

	t.Run("matching IPLD node is emitted", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		ctx := context.Background()
		n := node.MakeOfflineNode(t)
		api := New(n)

		ipldnode := types.NewBlockForTest(nil, 1234).ToNode()

		// put into out blockservice
		assert.NoError(n.BlockService().AddBlock(ipldnode))

		res, err := api.Dag().Get(ctx, ipldnode.Cid().String())
		assert.NoError(err)

		nodeBack, ok := res.(format.Node)
		assert.True(ok)
		assert.Equal(ipldnode.Cid().String(), nodeBack.Cid().String())
	})
}
