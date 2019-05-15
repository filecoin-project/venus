package core

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/config"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

var mockSigner, _ = types.NewMockSignersAndKeyInfo(10)
var newSignedMessage = types.NewSignedMessageForTestGetter(mockSigner)

func TestMessagePoolAddRemove(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	pool := NewMessagePool(th.NewTestMessagePoolAPI(0), config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
	msg1 := newSignedMessage()
	msg2 := mustSetNonce(mockSigner, newSignedMessage(), 1)

	c1, err := msg1.Cid()
	assert.NoError(t, err)
	c2, err := msg2.Cid()
	assert.NoError(t, err)

	assert.Len(t, pool.Pending(), 0)
	m, ok := pool.Get(c1)
	assert.Nil(t, m)
	assert.False(t, ok)

	_, err = pool.Add(ctx, msg1)
	assert.NoError(t, err)
	assert.Len(t, pool.Pending(), 1)

	_, err = pool.Add(ctx, msg2)
	assert.NoError(t, err)
	assert.Len(t, pool.Pending(), 2)

	m, ok = pool.Get(c1)
	assert.Equal(t, msg1, m)
	assert.True(t, ok)
	m, ok = pool.Get(c2)
	assert.Equal(t, msg2, m)
	assert.True(t, ok)

	pool.Remove(c1)
	assert.Len(t, pool.Pending(), 1)
	pool.Remove(c2)
	assert.Len(t, pool.Pending(), 0)
}

func TestMessagePoolValidate(t *testing.T) {
	tf.UnitTest(t)

	t.Run("message pool rejects messages after it reaches its limit", func(t *testing.T) {
		// pull the default size from the default config value
		mpoolCfg := config.NewDefaultConfig().Mpool
		maxMessagePoolSize := mpoolCfg.MaxPoolSize
		ctx := context.Background()
		pool := NewMessagePool(th.NewTestMessagePoolAPI(0), mpoolCfg, th.NewMockMessagePoolValidator())

		smsgs := types.NewSignedMsgs(maxMessagePoolSize+1, mockSigner)
		for _, smsg := range smsgs[:maxMessagePoolSize] {
			_, err := pool.Add(ctx, smsg)
			require.NoError(t, err)
		}

		assert.Len(t, pool.Pending(), maxMessagePoolSize)

		// attempt to add one more
		_, err := pool.Add(ctx, smsgs[maxMessagePoolSize])
		require.Error(t, err)
		assert.Contains(t, err.Error(), "message pool is full")

		assert.Len(t, pool.Pending(), maxMessagePoolSize)
	})

	t.Run("validates no two messages are added with same nonce", func(t *testing.T) {
		ctx := context.Background()
		pool := NewMessagePool(th.NewTestMessagePoolAPI(0), config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())

		smsg1 := newSignedMessage()
		_, err := pool.Add(ctx, smsg1)
		require.NoError(t, err)

		smsg2 := mustSetNonce(mockSigner, newSignedMessage(), smsg1.Nonce)
		_, err = pool.Add(ctx, smsg2)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "message with same actor and nonce")
	})

	t.Run("validates using supplied validator", func(t *testing.T) {
		ctx := context.Background()
		api := th.NewTestMessagePoolAPI(0)
		validator := th.NewMockMessagePoolValidator()
		validator.Valid = false
		pool := NewMessagePool(api, config.NewDefaultConfig().Mpool, validator)

		smsg1 := mustSetNonce(mockSigner, newSignedMessage(), 0)
		_, err := pool.Add(ctx, smsg1)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mock validation error")
	})
}

func TestMessagePoolDedup(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	pool := NewMessagePool(th.NewTestMessagePoolAPI(0), config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
	msg1 := newSignedMessage()

	assert.Len(t, pool.Pending(), 0)
	_, err := pool.Add(ctx, msg1)
	assert.NoError(t, err)
	assert.Len(t, pool.Pending(), 1)

	_, err = pool.Add(ctx, msg1)
	assert.NoError(t, err)
	assert.Len(t, pool.Pending(), 1)
}

func TestMessagePoolAsync(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	count := 400
	mpoolCfg := config.NewDefaultConfig().Mpool
	mpoolCfg.MaxPoolSize = count
	msgs := types.NewSignedMsgs(count, mockSigner)

	pool := NewMessagePool(th.NewTestMessagePoolAPI(0), mpoolCfg, th.NewMockMessagePoolValidator())
	var wg sync.WaitGroup

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			for j := 0; j < count/4; j++ {
				_, err := pool.Add(ctx, msgs[j+(count/4)*i])
				assert.NoError(t, err)
			}
			wg.Done()
		}(i)
	}

	wg.Wait()
	assert.Len(t, pool.Pending(), count)
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
func assertPoolEquals(t *testing.T, p *MessagePool, expMsgs ...*types.SignedMessage) {
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

func TestUpdateMessagePool(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	type msgs []*types.SignedMessage
	type msgsSet [][]*types.SignedMessage

	t.Run("Replace head", func(t *testing.T) {
		// Msg pool: [m0, m1], Chain: b[]
		// to
		// Msg pool: [m0],     Chain: b[m1]
		store := hamt.NewCborStore()
		p := NewMessagePool(th.NewTestMessagePoolAPI(0), config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())

		m := types.NewSignedMsgs(2, mockSigner)
		MustAdd(p, m[0], m[1])

		parent := types.TipSet{}

		blk := types.Block{Height: 0}
		parent[blk.Cid()] = &blk

		oldChain := NewChainWithMessages(store, parent, msgsSet{})
		oldTipSet := headOf(oldChain)

		newChain := NewChainWithMessages(store, parent, msgsSet{msgs{m[1]}})
		newTipSet := headOf(newChain)

		assert.NoError(t, p.UpdateMessagePool(ctx, &storeBlockProvider{store}, oldTipSet, newTipSet))
		assertPoolEquals(t, p, m[0])
	})

	t.Run("Replace head with self", func(t *testing.T) {
		// Msg pool: [m0, m1], Chain: b[m2]
		// to
		// Msg pool: [m0, m1], Chain: b[m2]
		store := hamt.NewCborStore()
		p := NewMessagePool(th.NewTestMessagePoolAPI(0), config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())

		m := types.NewSignedMsgs(3, mockSigner)
		MustAdd(p, m[0], m[1])

		oldChain := NewChainWithMessages(store, types.TipSet{}, msgsSet{msgs{m[2]}})
		oldTipSet := headOf(oldChain)

		assert.NoError(t, p.UpdateMessagePool(ctx, &storeBlockProvider{store}, oldTipSet, oldTipSet)) // sic
		assertPoolEquals(t, p, m[0], m[1])
	})

	t.Run("Replace head with a long chain", func(t *testing.T) {
		// Msg pool: [m2, m5],     Chain: b[m0, m1]
		// to
		// Msg pool: [m1],         Chain: b[m2, m3] -> b[m4] -> b[m0] -> b[] -> b[m5, m6]
		store := hamt.NewCborStore()
		p := NewMessagePool(th.NewTestMessagePoolAPI(0), config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())

		m := types.NewSignedMsgs(7, mockSigner)
		MustAdd(p, m[2], m[5])

		parent := types.TipSet{}

		blk := types.Block{Height: 0}
		parent[blk.Cid()] = &blk
		oldChain := NewChainWithMessages(store, parent, msgsSet{msgs{m[0], m[1]}})
		oldTipSet := headOf(oldChain)

		newChain := NewChainWithMessages(store, parent,
			msgsSet{msgs{m[2], m[3]}},
			msgsSet{msgs{m[4]}},
			msgsSet{msgs{m[0]}},
			msgsSet{msgs{}},
			msgsSet{msgs{m[5], m[6]}},
		)
		newTipSet := headOf(newChain)

		assert.NoError(t, p.UpdateMessagePool(ctx, &storeBlockProvider{store}, oldTipSet, newTipSet))
		assertPoolEquals(t, p, m[1])
	})

	t.Run("Replace head with multi-block tipset chains", func(t *testing.T) {
		// Msg pool: [m2, m5],     Chain: {b[m0], b[m1]}
		// to
		// Msg pool: [m1],         Chain: b[m2, m3] -> {b[m4], b[m0], b[], b[]} -> {b[], b[m6,m5]}
		store := hamt.NewCborStore()
		p := NewMessagePool(th.NewTestMessagePoolAPI(0), config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())

		m := types.NewSignedMsgs(7, mockSigner)
		MustAdd(p, m[2], m[5])

		parent := types.TipSet{}

		blk := types.Block{Height: 0}
		parent[blk.Cid()] = &blk

		oldChain := NewChainWithMessages(store, parent, msgsSet{msgs{m[0]}, msgs{m[1]}})
		oldTipSet := headOf(oldChain)

		newChain := NewChainWithMessages(store, parent,
			msgsSet{msgs{m[2], m[3]}},
			msgsSet{msgs{m[4]}, msgs{m[0]}, msgs{}, msgs{}},
			msgsSet{msgs{}, msgs{m[5], m[6]}},
		)
		newTipSet := headOf(newChain)

		assert.NoError(t, p.UpdateMessagePool(ctx, &storeBlockProvider{store}, oldTipSet, newTipSet))
		assertPoolEquals(t, p, m[1])
	})

	t.Run("Replace internal node (second one)", func(t *testing.T) {
		// Msg pool: [m3, m5],     Chain: b[m0] -> b[m1] -> b[m2]
		// to
		// Msg pool: [m1, m2],     Chain: b[m0] -> b[m3] -> b[m4, m5]
		store := hamt.NewCborStore()
		p := NewMessagePool(th.NewTestMessagePoolAPI(0), config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())

		m := types.NewSignedMsgs(6, mockSigner)
		MustAdd(p, m[3], m[5])

		oldChain := NewChainWithMessages(store, types.TipSet{}, msgsSet{msgs{m[0]}}, msgsSet{msgs{m[1]}}, msgsSet{msgs{m[2]}})
		oldTipSet := headOf(oldChain)

		newChain := NewChainWithMessages(store, oldChain[0], msgsSet{msgs{m[3]}}, msgsSet{msgs{m[4], m[5]}})
		newTipSet := headOf(newChain)

		assert.NoError(t, p.UpdateMessagePool(ctx, &storeBlockProvider{store}, oldTipSet, newTipSet))
		assertPoolEquals(t, p, m[1], m[2])
	})

	t.Run("Replace internal node (second one) with a long chain", func(t *testing.T) {
		// Msg pool: [m6],         Chain: b[m0] -> b[m1] -> b[m2]
		// to
		// Msg pool: [m6],         Chain: b[m0] -> b[m3] -> b[m4] -> b[m5] -> b[m1, m2]
		store := hamt.NewCborStore()
		p := NewMessagePool(th.NewTestMessagePoolAPI(0), config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())

		m := types.NewSignedMsgs(7, mockSigner)
		MustAdd(p, m[6])

		oldChain := NewChainWithMessages(store, types.TipSet{},
			msgsSet{msgs{m[0]}},
			msgsSet{msgs{m[1]}},
			msgsSet{msgs{m[2]}},
		)
		oldTipSet := headOf(oldChain)

		newChain := NewChainWithMessages(store, oldChain[0],
			msgsSet{msgs{m[3]}},
			msgsSet{msgs{m[4]}},
			msgsSet{msgs{m[5]}},
			msgsSet{msgs{m[1], m[2]}},
		)
		newTipSet := headOf(newChain)

		assert.NoError(t, p.UpdateMessagePool(ctx, &storeBlockProvider{store}, oldTipSet, newTipSet))
		assertPoolEquals(t, p, m[6])
	})

	t.Run("Replace internal node with multi-block tipset chains", func(t *testing.T) {
		// Msg pool: [m6],         Chain: {b[m0], b[m1]} -> b[m2]
		// to
		// Msg pool: [m6],         Chain: {b[m0], b[m1]} -> b[m3] -> b[m4] -> {b[m5], b[m1, m2]}
		store := hamt.NewCborStore()
		p := NewMessagePool(th.NewTestMessagePoolAPI(0), config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())

		m := types.NewSignedMsgs(7, mockSigner)
		MustAdd(p, m[6])

		oldChain := NewChainWithMessages(store, types.TipSet{},
			msgsSet{msgs{m[0]}, msgs{m[1]}},
			msgsSet{msgs{m[2]}},
		)
		oldTipSet := headOf(oldChain)

		newChain := NewChainWithMessages(store, oldChain[0],
			msgsSet{msgs{m[3]}},
			msgsSet{msgs{m[4]}},
			msgsSet{msgs{m[5]}, msgs{m[1], m[2]}},
		)
		newTipSet := headOf(newChain)

		assert.NoError(t, p.UpdateMessagePool(ctx, &storeBlockProvider{store}, oldTipSet, newTipSet))
		assertPoolEquals(t, p, m[6])
	})

	t.Run("Replace with same messages in different block structure", func(t *testing.T) {
		// Msg pool: [m3, m5],     Chain: b[m0] -> b[m1] -> b[m2]
		// to
		// Msg pool: [m3, m5],     Chain: {b[m0], b[m1], b[m2]}
		store := hamt.NewCborStore()
		p := NewMessagePool(th.NewTestMessagePoolAPI(0), config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())

		m := types.NewSignedMsgs(6, mockSigner)
		MustAdd(p, m[3], m[5])

		parent := types.TipSet{}

		blk := types.Block{Height: 0}
		parent[blk.Cid()] = &blk

		oldChain := NewChainWithMessages(store, parent,
			msgsSet{msgs{m[0]}},
			msgsSet{msgs{m[1]}},
			msgsSet{msgs{m[2]}},
		)
		oldTipSet := headOf(oldChain)

		newChain := NewChainWithMessages(store, parent,
			msgsSet{msgs{m[0]}, msgs{m[1]}, msgs{m[2]}},
		)
		newTipSet := headOf(newChain)

		assert.NoError(t, p.UpdateMessagePool(ctx, &storeBlockProvider{store}, oldTipSet, newTipSet))
		assertPoolEquals(t, p, m[3], m[5])
	})

	t.Run("Truncate to internal node", func(t *testing.T) {
		// Msg pool: [],               Chain: b[m0] -> b[m1] -> b[m2] -> b[m3]
		// to
		// Msg pool: [m2, m3],         Chain: b[m0] -> b[m1]
		store := hamt.NewCborStore()
		p := NewMessagePool(th.NewTestMessagePoolAPI(0), config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
		m := types.NewSignedMsgs(4, mockSigner)

		oldChain := NewChainWithMessages(store, types.TipSet{},
			msgsSet{msgs{m[0]}},
			msgsSet{msgs{m[1]}},
			msgsSet{msgs{m[2]}},
			msgsSet{msgs{m[3]}},
		)
		oldTipSet := headOf(oldChain)

		oldTipSetPrev := oldChain[1]
		assert.NoError(t, p.UpdateMessagePool(ctx, &storeBlockProvider{store}, oldTipSet, oldTipSetPrev))
		assertPoolEquals(t, p, m[2], m[3])
	})

	t.Run("Extend head", func(t *testing.T) {
		// Msg pool: [m0, m1], Chain: b[]
		// to
		// Msg pool: [m0],     Chain: b[] -> b[m1, m2]
		store := hamt.NewCborStore()
		p := NewMessagePool(th.NewTestMessagePoolAPI(0), config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())

		m := types.NewSignedMsgs(3, mockSigner)
		MustAdd(p, m[0], m[1])

		oldChain := NewChainWithMessages(store, types.TipSet{}, msgsSet{msgs{}})
		oldTipSet := headOf(oldChain)

		newChain := NewChainWithMessages(store, oldChain[len(oldChain)-1], msgsSet{msgs{m[1], m[2]}})
		newTipSet := headOf(newChain)

		assert.NoError(t, p.UpdateMessagePool(ctx, &storeBlockProvider{store}, oldTipSet, newTipSet))
		assertPoolEquals(t, p, m[0])
	})

	t.Run("Extend head with a longer chain and more messages", func(t *testing.T) {
		// Msg pool: [m2, m5],     Chain: b[m0] -> b[m1]
		// to
		// Msg pool: [],           Chain: b[m0] -> b[m1] -> b[m2, m3] -> b[m4] -> b[m5, m6]
		store := hamt.NewCborStore()
		p := NewMessagePool(th.NewTestMessagePoolAPI(0), config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())

		m := types.NewSignedMsgs(7, mockSigner)
		MustAdd(p, m[2], m[5])

		oldChain := NewChainWithMessages(store, types.TipSet{}, msgsSet{msgs{m[0]}}, msgsSet{msgs{m[1]}})
		oldTipSet := headOf(oldChain)

		newChain := NewChainWithMessages(store, oldChain[1],
			msgsSet{msgs{m[2], m[3]}},
			msgsSet{msgs{m[4]}},
			msgsSet{msgs{m[5], m[6]}},
		)
		newTipSet := headOf(newChain)

		assert.NoError(t, p.UpdateMessagePool(ctx, &storeBlockProvider{store}, oldTipSet, newTipSet))
		assertPoolEquals(t, p)
	})

	t.Run("Times out old messages", func(t *testing.T) {
		var err error
		store := hamt.NewCborStore()
		api := th.NewTestMessagePoolAPI(0)
		p := NewMessagePool(api, config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())

		m := types.NewSignedMsgs(MessageTimeOut, mockSigner)

		head := headOf(NewChainWithMessages(store, types.TipSet{}, msgsSet{msgs{}}))

		// Add a message at each block height until MessageTimeOut is reached
		for i := 0; i < MessageTimeOut; i++ {
			// api.Height determines block time at which message is added
			api.Height, err = head.Height()
			require.NoError(t, err)

			MustAdd(p, m[i])

			// update pool with tipset that has no messages
			next := headOf(NewChainWithMessages(store, head, msgsSet{msgs{}}))
			assert.NoError(t, p.UpdateMessagePool(ctx, &storeBlockProvider{store}, head, next))

			// assert all added messages still in pool
			assertPoolEquals(t, p, m[:i+1]...)

			head = next
		}

		// next tipset times out first message only
		next := headOf(NewChainWithMessages(store, head, msgsSet{msgs{}}))
		assert.NoError(t, p.UpdateMessagePool(ctx, &storeBlockProvider{store}, head, next))
		assertPoolEquals(t, p, m[1:]...)

		// adding a chain of multiple tipsets times out based on final state
		for i := 0; i < 4; i++ {
			next = headOf(NewChainWithMessages(store, next, msgsSet{msgs{}}))
		}
		assert.NoError(t, p.UpdateMessagePool(ctx, &storeBlockProvider{store}, head, next))
		assertPoolEquals(t, p, m[5:]...)
	})

	t.Run("Message timeout is unaffected by null tipsets", func(t *testing.T) {
		var err error
		store := hamt.NewCborStore()
		blockTimer := th.NewTestMessagePoolAPI(0)
		p := NewMessagePool(blockTimer, config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())

		m := types.NewSignedMsgs(MessageTimeOut, mockSigner)

		head := headOf(NewChainWithMessages(store, types.TipSet{}, msgsSet{msgs{}}))

		// Add a message at each block height until MessageTimeOut is reached
		for i := 0; i < MessageTimeOut; i++ {
			// blockTimer.Height determines block time at which message is added
			blockTimer.Height, err = head.Height()
			require.NoError(t, err)

			MustAdd(p, m[i])

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
			MustPut(store, blk)
			next[blk.Cid()] = blk

			assert.NoError(t, p.UpdateMessagePool(ctx, &storeBlockProvider{store}, head, next))

			// assert all added messages still in pool
			assertPoolEquals(t, p, m[:i+1]...)

			head = next
		}

		// next tipset times out first message only
		next := headOf(NewChainWithMessages(store, head, msgsSet{msgs{}}))
		assert.NoError(t, p.UpdateMessagePool(ctx, &storeBlockProvider{store}, head, next))
		assertPoolEquals(t, p, m[1:]...)
	})
}

func TestLargestNonce(t *testing.T) {
	tf.UnitTest(t)

	t.Run("No matches", func(t *testing.T) {
		p := NewMessagePool(th.NewTestMessagePoolAPI(0), config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())

		m := types.NewSignedMsgs(2, mockSigner)
		MustAdd(p, m[0], m[1])

		_, found := p.LargestNonce(address.NewForTestGetter()())
		assert.False(t, found)
	})

	t.Run("Match, largest is zero", func(t *testing.T) {
		p := NewMessagePool(th.NewTestMessagePoolAPI(0), config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())

		m := types.NewMsgsWithAddrs(1, mockSigner.Addresses)
		m[0].Nonce = 0

		sm, err := types.SignMsgs(mockSigner, m)
		require.NoError(t, err)

		MustAdd(p, sm...)

		largest, found := p.LargestNonce(m[0].From)
		assert.True(t, found)
		assert.Equal(t, uint64(0), largest)
	})

	t.Run("Match", func(t *testing.T) {
		p := NewMessagePool(th.NewTestMessagePoolAPI(0), config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())

		m := types.NewMsgsWithAddrs(3, mockSigner.Addresses)
		m[1].Nonce = 1
		m[2].Nonce = 2
		m[2].From = m[1].From

		sm, err := types.SignMsgs(mockSigner, m)
		require.NoError(t, err)

		MustAdd(p, sm...)

		largest, found := p.LargestNonce(m[2].From)
		assert.True(t, found)
		assert.Equal(t, uint64(2), largest)
	})
}

type storeBlockProvider struct {
	store *hamt.CborIpldStore
}

func (p *storeBlockProvider) GetBlock(ctx context.Context, cid cid.Cid) (*types.Block, error) {
	var blk types.Block
	if err := p.store.Get(ctx, cid, &blk); err != nil {
		return nil, errors.Wrapf(err, "failed to get block %s", cid)
	}
	return &blk, nil
}

func mustSetNonce(signer types.Signer, message *types.SignedMessage, nonce types.Uint64) *types.SignedMessage {
	return mustResignMessage(signer, message, func(m *types.Message) {
		m.Nonce = nonce
	})
}

func mustResignMessage(signer types.Signer, message *types.SignedMessage, f func(*types.Message)) *types.SignedMessage {
	var msg types.Message
	msg = message.Message
	f(&msg)
	smg, err := signMessage(signer, msg)
	if err != nil {
		panic("Error signing message")
	}
	return smg
}

func signMessage(signer types.Signer, message types.Message) (*types.SignedMessage, error) {
	return types.NewSignedMessage(message, signer, types.NewGasPrice(0), types.NewGasUnits(0))
}
