package core_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/core"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
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
		builder, chainProvider := newBuilderAndProvider(t)
		p := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := core.NewInbox(p, 10, chainProvider, builder)

		m := types.NewSignedMsgs(2, mockSigner)
		mustAdd(ib, m[0], m[1])

		parent := chainProvider.headTip()
		oldChain := core.RequireChainWithMessages(t, builder, parent, msgsSet{})
		oldTipSet := headOf(oldChain)

		newChain := core.RequireChainWithMessages(t, builder, parent, msgsSet{msgs{m[1]}})
		newTipSet := headOf(newChain)

		assert.NoError(t, ib.HandleNewHead(ctx, oldTipSet, newTipSet))
		assertPoolEquals(t, p, m[0])
	})

	t.Run("Replace head with self", func(t *testing.T) {
		// Msg pool: [m0, m1], Chain: b[m2]
		// to
		// Msg pool: [m0, m1], Chain: b[m2]
		builder, chainProvider := newBuilderAndProvider(t)
		p := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := core.NewInbox(p, 10, chainProvider, builder)

		m := types.NewSignedMsgs(3, mockSigner)
		mustAdd(ib, m[0], m[1])

		parent := chainProvider.headTip()
		oldChain := core.RequireChainWithMessages(t, builder, parent, msgsSet{msgs{m[2]}})
		oldTipSet := headOf(oldChain)

		assert.NoError(t, ib.HandleNewHead(ctx, oldTipSet, oldTipSet)) // sic
		assertPoolEquals(t, p, m[0], m[1])
	})

	t.Run("Replace head with a long chain", func(t *testing.T) {
		// Msg pool: [m2, m5],     Chain: b[m0, m1]
		// to
		// Msg pool: [m1],         Chain: b[m2, m3] -> b[m4] -> b[m0] -> b[] -> b[m5, m6]
		builder, chainProvider := newBuilderAndProvider(t)
		p := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := core.NewInbox(p, 10, chainProvider, builder)

		m := types.NewSignedMsgs(7, mockSigner)
		mustAdd(ib, m[2], m[5])

		parent := chainProvider.headTip()
		oldChain := core.RequireChainWithMessages(t, builder, parent, msgsSet{msgs{m[0], m[1]}})
		oldTipSet := headOf(oldChain)

		newChain := core.RequireChainWithMessages(t, builder, parent,
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
		builder, chainProvider := newBuilderAndProvider(t)
		p := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := core.NewInbox(p, 10, chainProvider, builder)

		m := types.NewSignedMsgs(7, mockSigner)
		mustAdd(ib, m[2], m[5])

		parent := chainProvider.headTip()

		oldChain := core.RequireChainWithMessages(t, builder, parent, msgsSet{msgs{m[0]}, msgs{m[1]}})
		oldTipSet := headOf(oldChain)

		newChain := core.RequireChainWithMessages(t, builder, parent,
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
		builder, chainProvider := newBuilderAndProvider(t)
		p := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := core.NewInbox(p, 10, chainProvider, builder)

		m := types.NewSignedMsgs(6, mockSigner)
		mustAdd(ib, m[3], m[5])

		parent := chainProvider.headTip()
		oldChain := core.RequireChainWithMessages(t, builder, parent, msgsSet{msgs{m[0]}}, msgsSet{msgs{m[1]}}, msgsSet{msgs{m[2]}})
		oldTipSet := headOf(oldChain)

		newChain := core.RequireChainWithMessages(t, builder, oldChain[0], msgsSet{msgs{m[3]}}, msgsSet{msgs{m[4], m[5]}})
		newTipSet := headOf(newChain)

		assert.NoError(t, ib.HandleNewHead(ctx, oldTipSet, newTipSet))
		assertPoolEquals(t, p, m[1], m[2])
	})

	t.Run("Replace internal node (second one) with a long chain", func(t *testing.T) {
		// Msg pool: [m6],         Chain: b[m0] -> b[m1] -> b[m2]
		// to
		// Msg pool: [m6],         Chain: b[m0] -> b[m3] -> b[m4] -> b[m5] -> b[m1, m2]
		builder, chainProvider := newBuilderAndProvider(t)
		p := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := core.NewInbox(p, 10, chainProvider, builder)

		m := types.NewSignedMsgs(7, mockSigner)
		mustAdd(ib, m[6])
		parent := chainProvider.headTip()
		oldChain := core.RequireChainWithMessages(t, builder, parent,
			msgsSet{msgs{m[0]}},
			msgsSet{msgs{m[1]}},
			msgsSet{msgs{m[2]}},
		)
		oldTipSet := headOf(oldChain)

		newChain := core.RequireChainWithMessages(t, builder, oldChain[0],
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
		builder, chainProvider := newBuilderAndProvider(t)
		p := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := core.NewInbox(p, 10, chainProvider, builder)

		m := types.NewSignedMsgs(7, mockSigner)
		mustAdd(ib, m[6])

		parent := chainProvider.headTip()
		oldChain := core.RequireChainWithMessages(t, builder, parent,
			msgsSet{msgs{m[0]}, msgs{m[1]}},
			msgsSet{msgs{m[2]}},
		)
		oldTipSet := headOf(oldChain)

		newChain := core.RequireChainWithMessages(t, builder, oldChain[0],
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
		builder, chainProvider := newBuilderAndProvider(t)
		p := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := core.NewInbox(p, 10, chainProvider, builder)

		m := types.NewSignedMsgs(6, mockSigner)
		mustAdd(ib, m[3], m[5])

		parent := chainProvider.headTip()
		oldChain := core.RequireChainWithMessages(t, builder, parent,
			msgsSet{msgs{m[0]}},
			msgsSet{msgs{m[1]}},
			msgsSet{msgs{m[2]}},
		)
		oldTipSet := headOf(oldChain)

		newChain := core.RequireChainWithMessages(t, builder, parent,
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
		builder, chainProvider := newBuilderAndProvider(t)
		p := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := core.NewInbox(p, 10, chainProvider, builder)
		m := types.NewSignedMsgs(4, mockSigner)

		parent := chainProvider.headTip()
		oldChain := core.RequireChainWithMessages(t, builder, parent,
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
		builder, chainProvider := newBuilderAndProvider(t)
		p := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := core.NewInbox(p, 10, chainProvider, builder)

		m := types.NewSignedMsgs(3, mockSigner)
		mustAdd(ib, m[0], m[1])

		parent := chainProvider.headTip()
		oldChain := core.RequireChainWithMessages(t, builder, parent, msgsSet{msgs{}})
		oldTipSet := headOf(oldChain)

		newChain := core.RequireChainWithMessages(t, builder, oldChain[len(oldChain)-1], msgsSet{msgs{m[1], m[2]}})
		newTipSet := headOf(newChain)

		assert.NoError(t, ib.HandleNewHead(ctx, oldTipSet, newTipSet))
		assertPoolEquals(t, p, m[0])
	})

	t.Run("Extend head with a longer chain and more messages", func(t *testing.T) {
		// Msg pool: [m2, m5],     Chain: b[m0] -> b[m1]
		// to
		// Msg pool: [],           Chain: b[m0] -> b[m1] -> b[m2, m3] -> b[m4] -> b[m5, m6]
		builder, chainProvider := newBuilderAndProvider(t)
		p := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := core.NewInbox(p, 10, chainProvider, builder)

		m := types.NewSignedMsgs(7, mockSigner)
		mustAdd(ib, m[2], m[5])

		parent := chainProvider.headTip()
		oldChain := core.RequireChainWithMessages(t, builder, parent, msgsSet{msgs{m[0]}}, msgsSet{msgs{m[1]}})
		oldTipSet := headOf(oldChain)

		newChain := core.RequireChainWithMessages(t, builder, oldChain[1],
			msgsSet{msgs{m[2], m[3]}},
			msgsSet{msgs{m[4]}},
			msgsSet{msgs{m[5], m[6]}},
		)
		newTipSet := headOf(newChain)

		assert.NoError(t, ib.HandleNewHead(ctx, oldTipSet, newTipSet))
		assertPoolEquals(t, p)
	})

	t.Run("Times out old messages", func(t *testing.T) {
		builder, chainProvider := newBuilderAndProvider(t)
		p := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		maxAge := uint(10)
		ib := core.NewInbox(p, maxAge, chainProvider, builder)

		m := types.NewSignedMsgs(maxAge, mockSigner)

		parent := chainProvider.headTip()
		head := headOf(core.RequireChainWithMessages(t, builder, parent, msgsSet{msgs{}}))

		// Add a message at each block height until maxAge is reached
		for i := uint(0); i < maxAge; i++ {
			// chainProvider's head determines block time at which message is added
			chainProvider.setHead(head)

			mustAdd(ib, m[i])

			// update pool with tipset that has no messages
			next := headOf(core.RequireChainWithMessages(t, builder, head, msgsSet{msgs{}}))
			assert.NoError(t, ib.HandleNewHead(ctx, head, next))

			// assert all added messages still in pool
			assertPoolEquals(t, p, m[:i+1]...)

			head = next
		}

		// next tipset times out first message only
		next := headOf(core.RequireChainWithMessages(t, builder, head, msgsSet{msgs{}}))
		assert.NoError(t, ib.HandleNewHead(ctx, head, next))
		assertPoolEquals(t, p, m[1:]...)

		// adding a chain of multiple tipsets times out based on final state
		for i := 0; i < 4; i++ {
			next = headOf(core.RequireChainWithMessages(t, builder, next, msgsSet{msgs{}}))
		}
		assert.NoError(t, ib.HandleNewHead(ctx, head, next))
		assertPoolEquals(t, p, m[5:]...)
	})

	t.Run("Message timeout is unaffected by null tipsets", func(t *testing.T) {
		builder, chainProvider := newBuilderAndProvider(t)
		p := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		maxAge := uint(10)
		ib := core.NewInbox(p, maxAge, chainProvider, builder)

		m := types.NewSignedMsgs(maxAge, mockSigner)
		parent := chainProvider.headTip()
		head := headOf(core.RequireChainWithMessages(t, builder, parent, msgsSet{msgs{}}))

		// Add a message at each block height until maxAge is reached
		for i := uint(0); i < maxAge; i++ {
			// chainProvider's head determines block time at which message is added
			chainProvider.setHead(head)

			mustAdd(ib, m[i])

			// update pool with tipset that has no messages and four
			// null blocks
			next := builder.BuildOn(head, func(bb *chain.BlockBuilder) {
				bb.IncHeight(types.Uint64(4)) // 4 null blocks
			})

			assert.NoError(t, ib.HandleNewHead(ctx, head, next))

			// assert all added messages still in pool
			assertPoolEquals(t, p, m[:i+1]...)

			head = next
		}

		// next tipset times out first message only
		next := headOf(core.RequireChainWithMessages(t, builder, head, msgsSet{msgs{}}))
		assert.NoError(t, ib.HandleNewHead(ctx, head, next))
		assertPoolEquals(t, p, m[1:]...)
	})
}

func newBuilderAndProvider(t *testing.T) (*chain.Builder, *fakeChainProvider) {
	builder := chain.NewBuilder(t, address.Address{})
	return builder, &fakeChainProvider{builder, builder.NewGenesis()}
}

type fakeChainProvider struct {
	*chain.Builder
	head types.TipSet
}

func (p *fakeChainProvider) GetHead() types.TipSetKey {
	return p.head.Key()
}

// head is a convenience method for getting test state.
func (p *fakeChainProvider) headTip() types.TipSet {
	return p.head
}

// setHead is a convenience method for setting test state.
func (p *fakeChainProvider) setHead(newHead types.TipSet) {
	p.head = newHead
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
