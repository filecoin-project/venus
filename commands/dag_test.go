package commands

import (
	"context"
	"testing"

	cmds "gx/ipfs/QmUf5GFfV2Be3UtSAPKDVkoRd1TwEBTmx9TSSCFGGjNgdQ/go-ipfs-cmds"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestDagGet(t *testing.T) {
	t.Parallel()
	t.Run("bad arg", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		_, err := testhelpers.RunCommand(dagGetCmd, []string{"awful"}, nil, &Env{})
		assert.Error(err)
		assert.Contains(err.Error(), "invalid 'ipfs ref' path")
	})

	t.Run("ILPD node not found results in error", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		someCid := types.SomeCid()

		getter := func(ctx context.Context, c *cid.Cid) (ipld.Node, error) {
			types.AssertCidsEqual(assert, someCid, c)
			return nil, ipld.ErrNotFound
		}

		emitter := NewMockEmitter(func(v interface{}) error {
			return nil
		})

		err := runDagGetByCid(context.Background(), getter, emitter.emit, someCid)
		assert.Error(err)
		assert.Contains(err.Error(), "not found")
		assert.Equal(0, len(emitter.calls()))
	})

	t.Run("matching IPLD node is emitted", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		ipldnode := types.NewBlockForTest(nil, 1234).ToNode()

		emitter := NewMockEmitter(func(v interface{}) error {
			types.AssertHaveSameCid(assert, ipldnode, v.(cmds.Single).Value.(ipld.Node))
			return nil
		})

		getter := func(ctx context.Context, c *cid.Cid) (ipld.Node, error) {
			types.AssertCidsEqual(assert, ipldnode.Cid(), c)
			return ipldnode, nil
		}

		err := runDagGetByCid(context.Background(), getter, emitter.emit, ipldnode.Cid())
		assert.NoError(err)
		assert.Equal(1, len(emitter.calls()))
	})
}
