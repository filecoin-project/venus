package message_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
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
		chainProvider, parent := newProviderWithGenesis(t)
		p := message.NewPool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := message.NewInbox(p, 10, chainProvider, chainProvider)

		m := types.NewSignedMsgs(2, mockSigner)
		requireAdd(t, ib, m[0], m[1])

		oldChain := requireChainWithMessages(t, chainProvider.Builder, parent, msgsSet{})

		newChain := requireChainWithMessages(t, chainProvider.Builder, parent, msgsSet{msgs{m[1]}})

		assert.NoError(t, ib.HandleNewHead(ctx, oldChain, newChain))
		assertPoolEquals(t, p, m[0])
	})

	t.Run("Replace head with self", func(t *testing.T) {
		// Msg pool: [m0, m1], Chain: b[m2]
		// to
		// Msg pool: [m0, m1], Chain: b[m2]
		chainProvider, parent := newProviderWithGenesis(t)
		p := message.NewPool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := message.NewInbox(p, 10, chainProvider, chainProvider)

		m := types.NewSignedMsgs(3, mockSigner)
		requireAdd(t, ib, m[0], m[1])

		oldChain := requireChainWithMessages(t, chainProvider.Builder, parent, msgsSet{msgs{m[2]}})

		assert.NoError(t, ib.HandleNewHead(ctx, oldChain, oldChain)) // sic
		assertPoolEquals(t, p, m[0], m[1])
	})

	t.Run("Replace head with a long chain", func(t *testing.T) {
		// Msg pool: [m2, m5],     Chain: b[m0, m1]
		// to
		// Msg pool: [m1],         Chain: b[m2, m3] -> b[m4] -> b[m0] -> b[] -> b[m5, m6]
		chainProvider, parent := newProviderWithGenesis(t)
		p := message.NewPool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := message.NewInbox(p, 10, chainProvider, chainProvider)

		m := types.NewSignedMsgs(7, mockSigner)
		requireAdd(t, ib, m[2], m[5])

		oldChain := requireChainWithMessages(t, chainProvider.Builder, parent, msgsSet{msgs{m[0], m[1]}})

		newChain := requireChainWithMessages(t, chainProvider.Builder, parent,
			msgsSet{msgs{m[2], m[3]}},
			msgsSet{msgs{m[4]}},
			msgsSet{msgs{m[0]}},
			msgsSet{msgs{}},
			msgsSet{msgs{m[5], m[6]}},
		)

		assert.NoError(t, ib.HandleNewHead(ctx, oldChain, newChain))
		assertPoolEquals(t, p, m[1])
	})

	t.Run("Replace head with multi-block tipset chains", func(t *testing.T) {
		// Msg pool: [m2, m5],     Chain: {b[m0], b[m1]}
		// to
		// Msg pool: [m1],         Chain: b[m2, m3] -> {b[m4], b[m0], b[], b[]} -> {b[], b[m6,m5]}
		chainProvider, parent := newProviderWithGenesis(t)
		p := message.NewPool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := message.NewInbox(p, 10, chainProvider, chainProvider)

		m := types.NewSignedMsgs(7, mockSigner)
		requireAdd(t, ib, m[2], m[5])

		oldChain := requireChainWithMessages(t, chainProvider.Builder, parent, msgsSet{msgs{m[0]}, msgs{m[1]}})

		newChain := requireChainWithMessages(t, chainProvider.Builder, parent,
			msgsSet{msgs{m[2], m[3]}},
			msgsSet{msgs{m[4]}, msgs{m[0]}, msgs{}, msgs{}},
			msgsSet{msgs{}, msgs{m[5], m[6]}},
		)

		assert.NoError(t, ib.HandleNewHead(ctx, oldChain, newChain))
		assertPoolEquals(t, p, m[1])
	})

	t.Run("Replace internal node (second one)", func(t *testing.T) {
		// Msg pool: [m3, m5],     Chain: b[m0] -> b[m1] -> b[m2]
		// to
		// Msg pool: [m1, m2],     Chain: b[m0] -> b[m3] -> b[m4, m5]
		chainProvider, parent := newProviderWithGenesis(t)
		p := message.NewPool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := message.NewInbox(p, 10, chainProvider, chainProvider)

		m := types.NewSignedMsgs(6, mockSigner)
		requireAdd(t, ib, m[3], m[5])

		oldChain := requireChainWithMessages(t, chainProvider.Builder, parent,
			msgsSet{msgs{m[0]}},
			msgsSet{msgs{m[1]}},
			msgsSet{msgs{m[2]}},
		)

		newChain := requireChainWithMessages(t, chainProvider.Builder, oldChain[0],
			msgsSet{msgs{m[3]}},
			msgsSet{msgs{m[4], m[5]}},
		)

		assert.NoError(t, ib.HandleNewHead(ctx, oldChain[:len(oldChain)-1], newChain))
		assertPoolEquals(t, p, m[1], m[2])
	})

	t.Run("Replace internal node (second one) with a long chain", func(t *testing.T) {
		// Msg pool: [m6],         Chain: b[m0] -> b[m1] -> b[m2]
		// to
		// Msg pool: [m6],         Chain: b[m0] -> b[m3] -> b[m4] -> b[m5] -> b[m1, m2]
		chainProvider, parent := newProviderWithGenesis(t)
		p := message.NewPool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := message.NewInbox(p, 10, chainProvider, chainProvider)

		m := types.NewSignedMsgs(7, mockSigner)
		requireAdd(t, ib, m[6])
		oldChain := requireChainWithMessages(t, chainProvider.Builder, parent,
			msgsSet{msgs{m[0]}},
			msgsSet{msgs{m[1]}},
			msgsSet{msgs{m[2]}},
		)

		newChain := requireChainWithMessages(t, chainProvider.Builder, oldChain[0],
			msgsSet{msgs{m[3]}},
			msgsSet{msgs{m[4]}},
			msgsSet{msgs{m[5]}},
			msgsSet{msgs{m[1], m[2]}},
		)

		assert.NoError(t, ib.HandleNewHead(ctx, oldChain[:len(oldChain)-1], newChain))
		assertPoolEquals(t, p, m[6])
	})

	t.Run("Replace internal node with multi-block tipset chains", func(t *testing.T) {
		// Msg pool: [m6],         Chain: {b[m0], b[m1]} -> b[m2]
		// to
		// Msg pool: [m6],         Chain: {b[m0], b[m1]} -> b[m3] -> b[m4] -> {b[m5], b[m1, m2]}
		chainProvider, parent := newProviderWithGenesis(t)
		p := message.NewPool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := message.NewInbox(p, 10, chainProvider, chainProvider)

		m := types.NewSignedMsgs(7, mockSigner)
		requireAdd(t, ib, m[6])

		oldChain := requireChainWithMessages(t, chainProvider.Builder, parent,
			msgsSet{msgs{m[0]}, msgs{m[1]}},
			msgsSet{msgs{m[2]}},
		)

		newChain := requireChainWithMessages(t, chainProvider.Builder, oldChain[0],
			msgsSet{msgs{m[3]}},
			msgsSet{msgs{m[4]}},
			msgsSet{msgs{m[5]}, msgs{m[1], m[2]}},
		)

		assert.NoError(t, ib.HandleNewHead(ctx, oldChain[:len(oldChain)-1], newChain))
		assertPoolEquals(t, p, m[6])
	})

	t.Run("Replace with same messages in different block structure", func(t *testing.T) {
		// Msg pool: [m3, m5],     Chain: b[m0] -> b[m1] -> b[m2]
		// to
		// Msg pool: [m3, m5],     Chain: {b[m0], b[m1], b[m2]}
		chainProvider, parent := newProviderWithGenesis(t)
		p := message.NewPool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := message.NewInbox(p, 10, chainProvider, chainProvider)

		m := types.NewSignedMsgs(6, mockSigner)
		requireAdd(t, ib, m[3], m[5])

		oldChain := requireChainWithMessages(t, chainProvider.Builder, parent,
			msgsSet{msgs{m[0]}},
			msgsSet{msgs{m[1]}},
			msgsSet{msgs{m[2]}},
		)

		newChain := requireChainWithMessages(t, chainProvider.Builder, parent,
			msgsSet{msgs{m[0]}, msgs{m[1]}, msgs{m[2]}},
		)

		assert.NoError(t, ib.HandleNewHead(ctx, oldChain, newChain))
		assertPoolEquals(t, p, m[3], m[5])
	})

	t.Run("Truncate to internal node", func(t *testing.T) {
		// Msg pool: [],               Chain: b[m0] -> b[m1] -> b[m2] -> b[m3]
		// to
		// Msg pool: [m2, m3],         Chain: b[m0] -> b[m1]
		chainProvider, parent := newProviderWithGenesis(t)
		p := message.NewPool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := message.NewInbox(p, 10, chainProvider, chainProvider)
		m := types.NewSignedMsgs(4, mockSigner)

		oldChain := requireChainWithMessages(t, chainProvider.Builder, parent,
			msgsSet{msgs{m[0]}},
			msgsSet{msgs{m[1]}},
			msgsSet{msgs{m[2]}},
			msgsSet{msgs{m[3]}},
		)

		assert.NoError(t, ib.HandleNewHead(ctx, oldChain[:2], nil))
		assertPoolEquals(t, p, m[2], m[3])
	})

	t.Run("Extend head", func(t *testing.T) {
		// Msg pool: [m0, m1], Chain: b[]
		// to
		// Msg pool: [m0],     Chain: b[] -> b[m1, m2]
		chainProvider, parent := newProviderWithGenesis(t)
		p := message.NewPool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := message.NewInbox(p, 10, chainProvider, chainProvider)

		m := types.NewSignedMsgs(3, mockSigner)
		requireAdd(t, ib, m[0], m[1])

		oldChain := requireChainWithMessages(t, chainProvider.Builder, parent, msgsSet{msgs{}})

		newChain := requireChainWithMessages(t, chainProvider.Builder, oldChain[0], msgsSet{msgs{m[1], m[2]}})

		assert.NoError(t, ib.HandleNewHead(ctx, nil, newChain))
		assertPoolEquals(t, p, m[0])
	})

	t.Run("Extend head with a longer chain and more messages", func(t *testing.T) {
		// Msg pool: [m2, m5],     Chain: b[m0] -> b[m1]
		// to
		// Msg pool: [],           Chain: b[m0] -> b[m1] -> b[m2, m3] -> b[m4] -> b[m5, m6]
		chainProvider, parent := newProviderWithGenesis(t)
		p := message.NewPool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := message.NewInbox(p, 10, chainProvider, chainProvider)

		m := types.NewSignedMsgs(7, mockSigner)
		requireAdd(t, ib, m[2], m[5])

		oldChain := requireChainWithMessages(t, chainProvider.Builder, parent,
			msgsSet{msgs{m[0]}},
			msgsSet{msgs{m[1]}},
		)

		newChain := requireChainWithMessages(t, chainProvider.Builder, oldChain[0],
			msgsSet{msgs{m[2], m[3]}},
			msgsSet{msgs{m[4]}},
			msgsSet{msgs{m[5], m[6]}},
		)

		assert.NoError(t, ib.HandleNewHead(ctx, nil, newChain))
		assertPoolEquals(t, p)
	})

	t.Run("Messages added to new chain are added at chain height", func(t *testing.T) {
		// Msg pool: [m2, m5],     Chain: b[m0] -> b[m1]
		// to
		// Msg pool: [],           Chain: b[m0] -> b[m1] -> b[m2, m3] -> b[m4] -> b[m5, m6]
		chainProvider, parent := newProviderWithGenesis(t)
		p := message.NewPool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		ib := message.NewInbox(p, 5, chainProvider, chainProvider)

		m := types.NewSignedMsgs(1, mockSigner)

		// old chain with one tipset containing one message
		oldChain := requireChainWithMessages(t, chainProvider.Builder, parent, msgsSet{msgs{m[0]}})

		// new chain with 10 tipsets and no messages
		newChain := requireChainWithMessages(t, chainProvider.Builder, parent,
			msgsSet{msgs{}}, msgsSet{msgs{}}, msgsSet{msgs{}}, msgsSet{msgs{}}, msgsSet{msgs{}},
			msgsSet{msgs{}}, msgsSet{msgs{}}, msgsSet{msgs{}}, msgsSet{msgs{}}, msgsSet{msgs{}},
		)

		// reorg the chain. Messages added more than 5 tipsets from the head will be immediately dropped
		assert.NoError(t, ib.HandleNewHead(ctx, oldChain, newChain))

		// expect the first message to still be in the chain because it is added at the new head
		assertPoolEquals(t, p, m[0])
	})

	t.Run("Times out old messages", func(t *testing.T) {
		chainProvider, parent := newProviderWithGenesis(t)
		p := message.NewPool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		maxAge := uint(10)
		ib := message.NewInbox(p, maxAge, chainProvider, chainProvider)

		m := types.NewSignedMsgs(maxAge, mockSigner)

		head := requireChainWithMessages(t, chainProvider.Builder, parent, msgsSet{msgs{}})[0]

		// Add a message at each block height until maxAge is reached.
		for i := uint(0); i < maxAge; i++ {
			// chainProvider's head determines block time at which message is added
			chainProvider.SetHead(head.Key())

			requireAdd(t, ib, m[i])

			// update pool with tipset that has no messages
			next := requireChainWithMessages(t, chainProvider.Builder, head, msgsSet{msgs{}})[0]
			assert.NoError(t, ib.HandleNewHead(ctx, nil, []block.TipSet{next}))

			// assert all added messages still in pool
			assertPoolEquals(t, p, m[:i+1]...)

			head = next
		}
		require.Equal(t, types.Uint64(11), head.At(0).Height)

		// next tipset times out first message only
		next := requireChainWithMessages(t, chainProvider.Builder, head, msgsSet{msgs{}})[0]
		assert.NoError(t, ib.HandleNewHead(ctx, nil, []block.TipSet{next}))
		assertPoolEquals(t, p, m[1:]...)

		// adding a chain of 4 tipsets times out based on final state
		newChain := requireChainWithMessages(t, chainProvider.Builder, next,
			msgsSet{msgs{}},
			msgsSet{msgs{}},
			msgsSet{msgs{}},
			msgsSet{msgs{}},
		)
		require.Equal(t, types.Uint64(16), newChain[0].At(0).Height)
		assert.NoError(t, ib.HandleNewHead(ctx, nil, newChain))
		assertPoolEquals(t, p, m[5:]...)
	})

	t.Run("UnsignedMessage timeout is unaffected by null tipsets", func(t *testing.T) {
		chainProvider, parent := newProviderWithGenesis(t)
		p := message.NewPool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		maxAge := uint(10)
		ib := message.NewInbox(p, maxAge, chainProvider, chainProvider)

		m := types.NewSignedMsgs(maxAge, mockSigner)
		head := requireChainWithMessages(t, chainProvider.Builder, parent, msgsSet{msgs{}})[0]

		// Add a message at each block height until maxAge is reached
		for i := uint(0); i < maxAge; i++ {
			// chainProvider's head determines block time at which message is added
			chainProvider.SetHead(head.Key())

			requireAdd(t, ib, m[i])

			// update pool with tipset that has no messages and four
			// null blocks
			next := chainProvider.BuildOneOn(head, func(bb *chain.BlockBuilder) {
				bb.IncHeight(types.Uint64(4)) // 4 null blocks
			})

			assert.NoError(t, ib.HandleNewHead(ctx, nil, []block.TipSet{next}))

			// assert all added messages still in pool
			assertPoolEquals(t, p, m[:i+1]...)

			head = next
		}

		// next tipset times out first message only
		next := requireChainWithMessages(t, chainProvider.Builder, head, msgsSet{msgs{}})[0]
		assert.NoError(t, ib.HandleNewHead(ctx, nil, []block.TipSet{next}))
		assertPoolEquals(t, p, m[1:]...)
	})
}

func newProviderWithGenesis(t *testing.T) (*message.FakeProvider, block.TipSet) {
	provider := message.NewFakeProvider(t)
	head := provider.Builder.NewGenesis()
	provider.SetHead(head.Key())
	return provider, head
}

func requireAdd(t *testing.T, ib *message.Inbox, msgs ...*types.SignedMessage) {
	ctx := context.Background()
	for _, m := range msgs {
		_, err := ib.Add(ctx, m)
		require.NoError(t, err)
	}
}

func msgAsString(msg *types.SignedMessage) string {
	// When using NewMessageForTestGetter msg.Method is set
	// to "msgN" so we print that (it will correspond
	// to a variable of the same name in the tests
	// below).
	return msg.Message.Method.String()
}

func msgsAsString(msgs []*types.SignedMessage) string {
	s := ""
	for _, m := range msgs {
		s = fmt.Sprintf("%s%s ", s, msgAsString(m))
	}
	return "[" + s + "]"
}

// assertPoolEquals returns true if p contains exactly the expected messages.
func assertPoolEquals(t *testing.T, p *message.Pool, expMsgs ...*types.SignedMessage) {
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

// requireChainWithMessages creates a chain of tipsets containing the given messages
// using the provided chain builder.  The builder stores the chain.  Note that
// each msgSet argument is a slice of message slices.  Each slice of slices
// goes into a successive tipset and each subslice goes into one tipset block.
// Precondition: the root tipset must be defined.  The chain of tipsets is
// returned in descending height order (head-first).
// TODO: move this onto the builder, #3110
func requireChainWithMessages(t *testing.T, builder *chain.Builder, root block.TipSet, msgSets ...[][]*types.SignedMessage) []block.TipSet {
	var tipSets []block.TipSet
	parent := root
	require.True(t, parent.Defined())

	for _, tsMsgSet := range msgSets {
		if len(tsMsgSet) == 0 {
			parent = builder.BuildOneOn(parent, nil)
		} else {
			parent = builder.Build(parent, len(tsMsgSet), msgBuild(t, tsMsgSet))
		}
		tipSets = append(tipSets, parent)
	}
	chain.Reverse(tipSets)
	return tipSets
}

// msgBuild takes in the msgSet dictating which messages go on which block of
// a test tipset and returns a build function that adds these messages to the
// correct block using the chain.Builder.
func msgBuild(t *testing.T, msgSet [][]*types.SignedMessage) func(*chain.BlockBuilder, int) {
	return func(bb *chain.BlockBuilder, i int) {
		require.True(t, i <= len(msgSet))
		bb.AddMessages(msgSet[i], []*types.UnsignedMessage{})
	}
}
