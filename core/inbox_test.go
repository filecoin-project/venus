package core_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/core"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateMessagePool(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	type msgs []*types.SignedMessage
	type msgsSet [][]*types.SignedMessage

	var mockSigner, _ = types.NewMockSignersAndKeyInfo(10)

	t.Run("Replace head", func(t *testing.T) {
		// Msg pool: [m0, m1], Chain: b[]
		// to
		// Msg pool: [m0],     Chain: b[m1]
		store, chainProvider := newStoreAndProvider(0)
		p := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := core.NewInbox(p, 10, chainProvider)

		m := types.NewSignedMsgs(2, mockSigner)
		mustAdd(ib, m[0], m[1])

		parent := types.TipSet{}

		blk := types.Block{Height: 0}
		parent[blk.Cid()] = &blk

		oldChain := core.NewChainWithMessages(store, parent, msgsSet{})
		oldTipSet := headOf(oldChain)

		newChain := core.NewChainWithMessages(store, parent, msgsSet{msgs{m[1]}})
		newTipSet := headOf(newChain)

		assert.NoError(t, ib.HandleNewHead(ctx, oldTipSet, newTipSet))
		assertPoolEquals(t, p, m[0])
	})

	t.Run("Replace head with self", func(t *testing.T) {
		// Msg pool: [m0, m1], Chain: b[m2]
		// to
		// Msg pool: [m0, m1], Chain: b[m2]
		store, chainProvider := newStoreAndProvider(0)
		p := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := core.NewInbox(p, 10, chainProvider)

		m := types.NewSignedMsgs(3, mockSigner)
		mustAdd(ib, m[0], m[1])

		oldChain := core.NewChainWithMessages(store, types.TipSet{}, msgsSet{msgs{m[2]}})
		oldTipSet := headOf(oldChain)

		assert.NoError(t, ib.HandleNewHead(ctx, oldTipSet, oldTipSet)) // sic
		assertPoolEquals(t, p, m[0], m[1])
	})

	t.Run("Replace head with a long chain", func(t *testing.T) {
		// Msg pool: [m2, m5],     Chain: b[m0, m1]
		// to
		// Msg pool: [m1],         Chain: b[m2, m3] -> b[m4] -> b[m0] -> b[] -> b[m5, m6]
		store, chainProvider := newStoreAndProvider(0)
		p := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := core.NewInbox(p, 10, chainProvider)

		m := types.NewSignedMsgs(7, mockSigner)
		mustAdd(ib, m[2], m[5])

		parent := types.TipSet{}

		blk := types.Block{Height: 0}
		parent[blk.Cid()] = &blk
		oldChain := core.NewChainWithMessages(store, parent, msgsSet{msgs{m[0], m[1]}})
		oldTipSet := headOf(oldChain)

		newChain := core.NewChainWithMessages(store, parent,
			msgsSet{msgs{m[2], m[3]}},
			msgsSet{msgs{m[4]}},
			msgsSet{msgs{m[0]}},
			msgsSet{msgs{}},
			msgsSet{msgs{m[5], m[6]}},
		)
		newTipSet := headOf(newChain)

		assert.NoError(t, ib.HandleNewHead(ctx, oldTipSet, newTipSet))
		assertPoolEquals(t, p, m[1])
	})

	t.Run("Replace head with multi-block tipset chains", func(t *testing.T) {
		// Msg pool: [m2, m5],     Chain: {b[m0], b[m1]}
		// to
		// Msg pool: [m1],         Chain: b[m2, m3] -> {b[m4], b[m0], b[], b[]} -> {b[], b[m6,m5]}
		store, chainProvider := newStoreAndProvider(0)
		p := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := core.NewInbox(p, 10, chainProvider)

		m := types.NewSignedMsgs(7, mockSigner)
		mustAdd(ib, m[2], m[5])

		parent := types.TipSet{}

		blk := types.Block{Height: 0}
		parent[blk.Cid()] = &blk

		oldChain := core.NewChainWithMessages(store, parent, msgsSet{msgs{m[0]}, msgs{m[1]}})
		oldTipSet := headOf(oldChain)

		newChain := core.NewChainWithMessages(store, parent,
			msgsSet{msgs{m[2], m[3]}},
			msgsSet{msgs{m[4]}, msgs{m[0]}, msgs{}, msgs{}},
			msgsSet{msgs{}, msgs{m[5], m[6]}},
		)
		newTipSet := headOf(newChain)

		assert.NoError(t, ib.HandleNewHead(ctx, oldTipSet, newTipSet))
		assertPoolEquals(t, p, m[1])
	})

	t.Run("Replace internal node (second one)", func(t *testing.T) {
		// Msg pool: [m3, m5],     Chain: b[m0] -> b[m1] -> b[m2]
		// to
		// Msg pool: [m1, m2],     Chain: b[m0] -> b[m3] -> b[m4, m5]
		store, chainProvider := newStoreAndProvider(0)
		p := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := core.NewInbox(p, 10, chainProvider)

		m := types.NewSignedMsgs(6, mockSigner)
		mustAdd(ib, m[3], m[5])

		oldChain := core.NewChainWithMessages(store, types.TipSet{}, msgsSet{msgs{m[0]}}, msgsSet{msgs{m[1]}}, msgsSet{msgs{m[2]}})
		oldTipSet := headOf(oldChain)

		newChain := core.NewChainWithMessages(store, oldChain[0], msgsSet{msgs{m[3]}}, msgsSet{msgs{m[4], m[5]}})
		newTipSet := headOf(newChain)

		assert.NoError(t, ib.HandleNewHead(ctx, oldTipSet, newTipSet))
		assertPoolEquals(t, p, m[1], m[2])
	})

	t.Run("Replace internal node (second one) with a long chain", func(t *testing.T) {
		// Msg pool: [m6],         Chain: b[m0] -> b[m1] -> b[m2]
		// to
		// Msg pool: [m6],         Chain: b[m0] -> b[m3] -> b[m4] -> b[m5] -> b[m1, m2]
		store, chainProvider := newStoreAndProvider(0)
		p := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := core.NewInbox(p, 10, chainProvider)

		m := types.NewSignedMsgs(7, mockSigner)
		mustAdd(ib, m[6])

		oldChain := core.NewChainWithMessages(store, types.TipSet{},
			msgsSet{msgs{m[0]}},
			msgsSet{msgs{m[1]}},
			msgsSet{msgs{m[2]}},
		)
		oldTipSet := headOf(oldChain)

		newChain := core.NewChainWithMessages(store, oldChain[0],
			msgsSet{msgs{m[3]}},
			msgsSet{msgs{m[4]}},
			msgsSet{msgs{m[5]}},
			msgsSet{msgs{m[1], m[2]}},
		)
		newTipSet := headOf(newChain)

		assert.NoError(t, ib.HandleNewHead(ctx, oldTipSet, newTipSet))
		assertPoolEquals(t, p, m[6])
	})

	t.Run("Replace internal node with multi-block tipset chains", func(t *testing.T) {
		// Msg pool: [m6],         Chain: {b[m0], b[m1]} -> b[m2]
		// to
		// Msg pool: [m6],         Chain: {b[m0], b[m1]} -> b[m3] -> b[m4] -> {b[m5], b[m1, m2]}
		store, chainProvider := newStoreAndProvider(0)
		p := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := core.NewInbox(p, 10, chainProvider)

		m := types.NewSignedMsgs(7, mockSigner)
		mustAdd(ib, m[6])

		oldChain := core.NewChainWithMessages(store, types.TipSet{},
			msgsSet{msgs{m[0]}, msgs{m[1]}},
			msgsSet{msgs{m[2]}},
		)
		oldTipSet := headOf(oldChain)

		newChain := core.NewChainWithMessages(store, oldChain[0],
			msgsSet{msgs{m[3]}},
			msgsSet{msgs{m[4]}},
			msgsSet{msgs{m[5]}, msgs{m[1], m[2]}},
		)
		newTipSet := headOf(newChain)

		assert.NoError(t, ib.HandleNewHead(ctx, oldTipSet, newTipSet))
		assertPoolEquals(t, p, m[6])
	})

	t.Run("Replace with same messages in different block structure", func(t *testing.T) {
		// Msg pool: [m3, m5],     Chain: b[m0] -> b[m1] -> b[m2]
		// to
		// Msg pool: [m3, m5],     Chain: {b[m0], b[m1], b[m2]}
		store, chainProvider := newStoreAndProvider(0)
		p := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := core.NewInbox(p, 10, chainProvider)

		m := types.NewSignedMsgs(6, mockSigner)
		mustAdd(ib, m[3], m[5])

		parent := types.TipSet{}

		blk := types.Block{Height: 0}
		parent[blk.Cid()] = &blk

		oldChain := core.NewChainWithMessages(store, parent,
			msgsSet{msgs{m[0]}},
			msgsSet{msgs{m[1]}},
			msgsSet{msgs{m[2]}},
		)
		oldTipSet := headOf(oldChain)

		newChain := core.NewChainWithMessages(store, parent,
			msgsSet{msgs{m[0]}, msgs{m[1]}, msgs{m[2]}},
		)
		newTipSet := headOf(newChain)

		assert.NoError(t, ib.HandleNewHead(ctx, oldTipSet, newTipSet))
		assertPoolEquals(t, p, m[3], m[5])
	})

	t.Run("Truncate to internal node", func(t *testing.T) {
		// Msg pool: [],               Chain: b[m0] -> b[m1] -> b[m2] -> b[m3]
		// to
		// Msg pool: [m2, m3],         Chain: b[m0] -> b[m1]
		store, chainProvider := newStoreAndProvider(0)
		p := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := core.NewInbox(p, 10, chainProvider)
		m := types.NewSignedMsgs(4, mockSigner)

		oldChain := core.NewChainWithMessages(store, types.TipSet{},
			msgsSet{msgs{m[0]}},
			msgsSet{msgs{m[1]}},
			msgsSet{msgs{m[2]}},
			msgsSet{msgs{m[3]}},
		)
		oldTipSet := headOf(oldChain)

		oldTipSetPrev := oldChain[1]
		assert.NoError(t, ib.HandleNewHead(ctx, oldTipSet, oldTipSetPrev))
		assertPoolEquals(t, p, m[2], m[3])
	})

	t.Run("Extend head", func(t *testing.T) {
		// Msg pool: [m0, m1], Chain: b[]
		// to
		// Msg pool: [m0],     Chain: b[] -> b[m1, m2]
		store, chainProvider := newStoreAndProvider(0)
		p := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := core.NewInbox(p, 10, chainProvider)

		m := types.NewSignedMsgs(3, mockSigner)
		mustAdd(ib, m[0], m[1])

		oldChain := core.NewChainWithMessages(store, types.TipSet{}, msgsSet{msgs{}})
		oldTipSet := headOf(oldChain)

		newChain := core.NewChainWithMessages(store, oldChain[len(oldChain)-1], msgsSet{msgs{m[1], m[2]}})
		newTipSet := headOf(newChain)

		assert.NoError(t, ib.HandleNewHead(ctx, oldTipSet, newTipSet))
		assertPoolEquals(t, p, m[0])
	})

	t.Run("Extend head with a longer chain and more messages", func(t *testing.T) {
		// Msg pool: [m2, m5],     Chain: b[m0] -> b[m1]
		// to
		// Msg pool: [],           Chain: b[m0] -> b[m1] -> b[m2, m3] -> b[m4] -> b[m5, m6]
		store, chainProvider := newStoreAndProvider(0)
		p := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := core.NewInbox(p, 10, chainProvider)

		m := types.NewSignedMsgs(7, mockSigner)
		mustAdd(ib, m[2], m[5])

		oldChain := core.NewChainWithMessages(store, types.TipSet{}, msgsSet{msgs{m[0]}}, msgsSet{msgs{m[1]}})
		oldTipSet := headOf(oldChain)

		newChain := core.NewChainWithMessages(store, oldChain[1],
			msgsSet{msgs{m[2], m[3]}},
			msgsSet{msgs{m[4]}},
			msgsSet{msgs{m[5], m[6]}},
		)
		newTipSet := headOf(newChain)

		assert.NoError(t, ib.HandleNewHead(ctx, oldTipSet, newTipSet))
		assertPoolEquals(t, p)
	})

	t.Run("Times out old messages", func(t *testing.T) {
		var err error
		store, chainProvider := newStoreAndProvider(0)
		p := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		maxAge := uint(10)
		ib := core.NewInbox(p, maxAge, chainProvider)

		m := types.NewSignedMsgs(maxAge, mockSigner)

		head := headOf(core.NewChainWithMessages(store, types.TipSet{}, msgsSet{msgs{}}))

		// Add a message at each block height until maxAge is reached
		for i := uint(0); i < maxAge; i++ {
			// api.Height determines block time at which message is added
			chainProvider.height, err = head.Height()
			require.NoError(t, err)

			mustAdd(ib, m[i])

			// update pool with tipset that has no messages
			next := headOf(core.NewChainWithMessages(store, head, msgsSet{msgs{}}))
			assert.NoError(t, ib.HandleNewHead(ctx, head, next))

			// assert all added messages still in pool
			assertPoolEquals(t, p, m[:i+1]...)

			head = next
		}

		// next tipset times out first message only
		next := headOf(core.NewChainWithMessages(store, head, msgsSet{msgs{}}))
		assert.NoError(t, ib.HandleNewHead(ctx, head, next))
		assertPoolEquals(t, p, m[1:]...)

		// adding a chain of multiple tipsets times out based on final state
		for i := 0; i < 4; i++ {
			next = headOf(core.NewChainWithMessages(store, next, msgsSet{msgs{}}))
		}
		assert.NoError(t, ib.HandleNewHead(ctx, head, next))
		assertPoolEquals(t, p, m[5:]...)
	})

	t.Run("Message timeout is unaffected by null tipsets", func(t *testing.T) {
		var err error
		store, chainProvider := newStoreAndProvider(0)
		p := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		maxAge := uint(10)
		ib := core.NewInbox(p, maxAge, chainProvider)

		m := types.NewSignedMsgs(maxAge, mockSigner)

		head := headOf(core.NewChainWithMessages(store, types.TipSet{}, msgsSet{msgs{}}))

		// Add a message at each block height until maxAge is reached
		for i := uint(0); i < maxAge; i++ {
			// blockTimer.Height determines block time at which message is added
			chainProvider.height, err = head.Height()
			require.NoError(t, err)

			mustAdd(ib, m[i])

			// update pool with tipset that has no messages
			height, err := head.Height()
			require.NoError(t, err)

			// create a tipset at given height with one block containing no messages
			next := types.TipSet{}
			nextHeight := types.Uint64(height + 5) // simulate 4 null blocks
			blk := &types.Block{
				Height:  nextHeight,
				Parents: head.ToSortedCidSet(),
			}
			core.MustPut(store, blk)
			next[blk.Cid()] = blk

			assert.NoError(t, ib.HandleNewHead(ctx, head, next))

			// assert all added messages still in pool
			assertPoolEquals(t, p, m[:i+1]...)

			head = next
		}

		// next tipset times out first message only
		next := headOf(core.NewChainWithMessages(store, head, msgsSet{msgs{}}))
		assert.NoError(t, ib.HandleNewHead(ctx, head, next))
		assertPoolEquals(t, p, m[1:]...)
	})
}

func newStoreAndProvider(height uint64) (*hamt.CborIpldStore, *fakeChainProvider) {
	store := hamt.NewCborStore()
	return store, &fakeChainProvider{height, store}
}

type fakeChainProvider struct {
	height uint64
	store  *hamt.CborIpldStore
}

func (p *fakeChainProvider) GetBlock(ctx context.Context, cid cid.Cid) (*types.Block, error) {
	var blk types.Block
	if err := p.store.Get(ctx, cid, &blk); err != nil {
		return nil, errors.Wrapf(err, "failed to get block %s", cid)
	}
	return &blk, nil
}

func (p *fakeChainProvider) BlockHeight() (uint64, error) {
	return p.height, nil
}

func mustAdd(ib *core.Inbox, msgs ...*types.SignedMessage) {
	ctx := context.Background()
	for _, m := range msgs {
		if _, err := ib.Add(ctx, m); err != nil {
			panic(err)
		}
	}
}

func msgAsString(msg *types.SignedMessage) string {
	// When using NewMessageForTestGetter msg.Method is set
	// to "msgN" so we print that (it will correspond
	// to a variable of the same name in the tests
	// below).
	return msg.Message.Method
}

func msgsAsString(msgs []*types.SignedMessage) string {
	s := ""
	for _, m := range msgs {
		s = fmt.Sprintf("%s%s ", s, msgAsString(m))
	}
	return "[" + s + "]"
}

// assertPoolEquals returns true if p contains exactly the expected messages.
func assertPoolEquals(t *testing.T, p *core.MessagePool, expMsgs ...*types.SignedMessage) {
	msgs := p.Pending()
	if len(msgs) != len(expMsgs) {
		assert.Failf(t, "wrong messages in pool", "expMsgs %v, got msgs %v", msgsAsString(expMsgs), msgsAsString(msgs))

	}
	for _, m1 := range expMsgs {
		found := false
		for _, m2 := range msgs {
			if types.SmsgCidsEqual(m1, m2) {
				found = true
				break
			}
		}
		if !found {
			assert.Failf(t, "wrong messages in pool", "expMsgs %v, got msgs %v (msgs doesn't contain %v)", msgsAsString(expMsgs), msgsAsString(msgs), msgAsString(m1))
		}
	}
}

func headOf(chain []types.TipSet) types.TipSet {
	return chain[len(chain)-1]
}
