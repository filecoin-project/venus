package core

import (
	"context"
	"fmt"
	hamt "gx/ipfs/QmcYBp5EDnJKfVN63F71rDTksvEf1cfijwCTWtw6bPG58T/go-hamt-ipld"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/types"
)

func TestMessagePoolAddRemove(t *testing.T) {
	assert := assert.New(t)
	newMsg := types.NewMessageForTestGetter()

	pool := NewMessagePool()
	msg1 := newMsg()
	msg2 := newMsg()

	c1, err := msg1.Cid()
	assert.NoError(err)
	c2, err := msg2.Cid()
	assert.NoError(err)

	assert.Len(pool.Pending(), 0)
	_, err = pool.Add(msg1)
	assert.NoError(err)
	assert.Len(pool.Pending(), 1)
	_, err = pool.Add(msg2)
	assert.NoError(err)
	assert.Len(pool.Pending(), 2)

	pool.Remove(c1)
	assert.Len(pool.Pending(), 1)
	pool.Remove(c2)
	assert.Len(pool.Pending(), 0)
}

func TestMessagePoolDedup(t *testing.T) {
	assert := assert.New(t)

	pool := NewMessagePool()
	msg1 := types.NewMessageForTestGetter()()

	assert.Len(pool.Pending(), 0)
	_, err := pool.Add(msg1)
	assert.NoError(err)
	assert.Len(pool.Pending(), 1)

	_, err = pool.Add(msg1)
	assert.NoError(err)
	assert.Len(pool.Pending(), 1)
}

func TestMessagePoolAsync(t *testing.T) {
	assert := assert.New(t)

	count := 400
	msgs := make([]*types.Message, count)
	addrGetter := types.NewAddressForTestGetter()

	for i := 0; i < count; i++ {
		msgs[i] = types.NewMessage(
			addrGetter(),
			addrGetter(),
			0,
			types.NewTokenAmount(1),
			"send",
			nil,
		)
	}

	pool := NewMessagePool()
	var wg sync.WaitGroup

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			for j := 0; j < count/4; j++ {
				_, err := pool.Add(msgs[j+(count/4)*i])
				assert.NoError(err)
			}
			wg.Done()
		}(i)
	}

	wg.Wait()
	assert.Len(pool.Pending(), count)
}

func msgAsString(msg *types.Message) string {
	// When using NewMessageForTestGetter msg.Method is set
	// to "msgN" so we print that (it will correspond
	// to a variable of the same name in the tests
	// below).
	return msg.Method
}

func msgsAsString(msgs []*types.Message) string {
	s := ""
	for _, m := range msgs {
		s = fmt.Sprintf("%s%s ", s, msgAsString(m))
	}
	return "[" + s + "]"
}

// assertPoolEquals returns true if p contains exactly the expected messages.
func assertPoolEquals(assert *assert.Assertions, p *MessagePool, expMsgs ...*types.Message) {
	msgs := p.Pending()
	if len(msgs) != len(expMsgs) {
		assert.Failf("wrong messages in pool", "expMsgs %v, got msgs %v", msgsAsString(expMsgs), msgsAsString(msgs))

	}
	for _, m1 := range expMsgs {
		found := false
		for _, m2 := range msgs {
			if types.MsgCidsEqual(m1, m2) {
				found = true
				break
			}
		}
		if !found {
			assert.Failf("wrong messages in pool", "expMsgs %v, got msgs %v (msgs doesn't contain %v)", msgsAsString(expMsgs), msgsAsString(msgs), msgAsString(m1))
		}
	}
}

func headOf(chain []TipSet) TipSet {
	return chain[len(chain)-1]
}

func TestUpdateMessagePool(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	type msgs []*types.Message
	type msgsSet [][]*types.Message

	t.Run("Replace head", func(t *testing.T) {
		// Msg pool: [m0, m1], Chain: b[]
		// to
		// Msg pool: [m0],     Chain: b[m1]
		store := hamt.NewCborStore()
		p := NewMessagePool()
		m := types.NewMsgs(2)
		MustAdd(p, m[0], m[1])
		oldChain := NewChainWithMessages(store, TipSet{}, msgsSet{})
		oldTipSet := headOf(oldChain)
		newChain := NewChainWithMessages(store, TipSet{}, msgsSet{msgs{m[1]}})
		newTipSet := headOf(newChain)
		assert.NoError(UpdateMessagePool(ctx, p, store, oldTipSet, newTipSet))
		assertPoolEquals(assert, p, m[0])
	})

	t.Run("Replace head with self", func(t *testing.T) {
		// Msg pool: [m0, m1], Chain: b[m2]
		// to
		// Msg pool: [m0, m1], Chain: b[m2]
		store := hamt.NewCborStore()
		p := NewMessagePool()
		m := types.NewMsgs(3)
		MustAdd(p, m[0], m[1])
		oldChain := NewChainWithMessages(store, TipSet{}, msgsSet{msgs{m[2]}})
		oldTipSet := headOf(oldChain)
		UpdateMessagePool(ctx, p, store, oldTipSet, oldTipSet) // sic
		assertPoolEquals(assert, p, m[0], m[1])
	})

	t.Run("Replace head with a long chain", func(t *testing.T) {
		// Msg pool: [m2, m5],     Chain: b[m0, m1]
		// to
		// Msg pool: [m1],         Chain: b[m2, m3] -> b[m4] -> b[m0] -> b[] -> b[m5, m6]
		store := hamt.NewCborStore()
		p := NewMessagePool()
		m := types.NewMsgs(7)
		MustAdd(p, m[2], m[5])
		oldChain := NewChainWithMessages(store, TipSet{}, msgsSet{msgs{m[0], m[1]}})
		oldTipSet := headOf(oldChain)
		newChain := NewChainWithMessages(store, TipSet{},
			msgsSet{msgs{m[2], m[3]}},
			msgsSet{msgs{m[4]}},
			msgsSet{msgs{m[0]}},
			msgsSet{msgs{}},
			msgsSet{msgs{m[5], m[6]}},
		)
		newTipSet := headOf(newChain)
		UpdateMessagePool(ctx, p, store, oldTipSet, newTipSet)
		assertPoolEquals(assert, p, m[1])
	})

	t.Run("Replace head with multi-block tipset chains", func(t *testing.T) {
		// Msg pool: [m2, m5],     Chain: {b[m0], b[m1]}
		// to
		// Msg pool: [m1],         Chain: b[m2, m3] -> {b[m4], b[m0], b[], b[]} -> {b[], b[m6,m5]}
		store := hamt.NewCborStore()
		p := NewMessagePool()
		m := types.NewMsgs(7)
		MustAdd(p, m[2], m[5])
		oldChain := NewChainWithMessages(store, TipSet{}, msgsSet{msgs{m[0]}, msgs{m[1]}})
		oldTipSet := headOf(oldChain)
		newChain := NewChainWithMessages(store, TipSet{},
			msgsSet{msgs{m[2], m[3]}},
			msgsSet{msgs{m[4]}, msgs{m[0]}, msgs{}, msgs{}},
			msgsSet{msgs{}, msgs{m[5], m[6]}},
		)
		newTipSet := headOf(newChain)
		UpdateMessagePool(ctx, p, store, oldTipSet, newTipSet)
		assertPoolEquals(assert, p, m[1])
	})

	t.Run("Replace internal node (second one)", func(t *testing.T) {
		// Msg pool: [m3, m5],     Chain: b[m0] -> b[m1] -> b[m2]
		// to
		// Msg pool: [m1, m2],     Chain: b[m0] -> b[m3] -> b[m4, m5]
		store := hamt.NewCborStore()
		p := NewMessagePool()
		m := types.NewMsgs(6)
		MustAdd(p, m[3], m[5])
		oldChain := NewChainWithMessages(store, TipSet{}, msgsSet{msgs{m[0]}}, msgsSet{msgs{m[1]}}, msgsSet{msgs{m[2]}})
		oldTipSet := headOf(oldChain)
		newChain := NewChainWithMessages(store, oldChain[0], msgsSet{msgs{m[3]}}, msgsSet{msgs{m[4], m[5]}})
		newTipSet := headOf(newChain)
		UpdateMessagePool(ctx, p, store, oldTipSet, newTipSet)
		assertPoolEquals(assert, p, m[1], m[2])
	})

	t.Run("Replace internal node (second one) with a long chain", func(t *testing.T) {
		// Msg pool: [m6],         Chain: b[m0] -> b[m1] -> b[m2]
		// to
		// Msg pool: [m6],         Chain: b[m0] -> b[m3] -> b[m4] -> b[m5] -> b[m1, m2]
		store := hamt.NewCborStore()
		p := NewMessagePool()
		m := types.NewMsgs(7)
		MustAdd(p, m[6])
		oldChain := NewChainWithMessages(store, TipSet{},
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
		UpdateMessagePool(ctx, p, store, oldTipSet, newTipSet)
		assertPoolEquals(assert, p, m[6])
	})

	t.Run("Replace internal node with multi-block tipset chains", func(t *testing.T) {
		// Msg pool: [m6],         Chain: {b[m0], b[m1]} -> b[m2]
		// to
		// Msg pool: [m6],         Chain: {b[m0], b[m1]} -> b[m3] -> b[m4] -> {b[m5], b[m1, m2]}
		store := hamt.NewCborStore()
		p := NewMessagePool()
		m := types.NewMsgs(7)
		MustAdd(p, m[6])
		oldChain := NewChainWithMessages(store, TipSet{},
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
		UpdateMessagePool(ctx, p, store, oldTipSet, newTipSet)
		assertPoolEquals(assert, p, m[6])
	})

	t.Run("Replace with same messages in different block structure", func(t *testing.T) {
		// Msg pool: [m3, m5],     Chain: b[m0] -> b[m1] -> b[m2]
		// to
		// Msg pool: [m3, m5],     Chain: {b[m0], b[m1], b[m2]}
		store := hamt.NewCborStore()
		p := NewMessagePool()
		m := types.NewMsgs(6)
		MustAdd(p, m[3], m[5])
		oldChain := NewChainWithMessages(store, TipSet{},
			msgsSet{msgs{m[0]}},
			msgsSet{msgs{m[1]}},
			msgsSet{msgs{m[2]}},
		)
		oldTipSet := headOf(oldChain)
		newChain := NewChainWithMessages(store, TipSet{},
			msgsSet{msgs{m[0]}, msgs{m[1]}, msgs{m[2]}},
		)
		newTipSet := headOf(newChain)
		UpdateMessagePool(ctx, p, store, oldTipSet, newTipSet)
		assertPoolEquals(assert, p, m[3], m[5])
	})

	t.Run("Truncate to internal node", func(t *testing.T) {
		// Msg pool: [],               Chain: b[m0] -> b[m1] -> b[m2] -> b[m3]
		// to
		// Msg pool: [m2, m3],         Chain: b[m0] -> b[m1]
		store := hamt.NewCborStore()
		p := NewMessagePool()
		m := types.NewMsgs(4)
		oldChain := NewChainWithMessages(store, TipSet{},
			msgsSet{msgs{m[0]}},
			msgsSet{msgs{m[1]}},
			msgsSet{msgs{m[2]}},
			msgsSet{msgs{m[3]}},
		)
		oldTipSet := headOf(oldChain)
		oldTipSetPrev := oldChain[1]
		UpdateMessagePool(ctx, p, store, oldTipSet, oldTipSetPrev)
		assertPoolEquals(assert, p, m[2], m[3])
	})

	t.Run("Extend head", func(t *testing.T) {
		// Msg pool: [m0, m1], Chain: b[]
		// to
		// Msg pool: [m0],     Chain: b[] -> b[m1, m2]
		store := hamt.NewCborStore()
		p := NewMessagePool()
		m := types.NewMsgs(3)
		MustAdd(p, m[0], m[1])
		oldChain := NewChainWithMessages(store, TipSet{}, msgsSet{msgs{}})
		oldTipSet := headOf(oldChain)
		newChain := NewChainWithMessages(store, oldChain[len(oldChain)-1], msgsSet{msgs{m[1], m[2]}})
		newTipSet := headOf(newChain)
		UpdateMessagePool(ctx, p, store, oldTipSet, newTipSet)
		assertPoolEquals(assert, p, m[0])
	})

	t.Run("Extend head with a longer chain and more messages", func(t *testing.T) {
		// Msg pool: [m2, m5],     Chain: b[m0] -> b[m1]
		// to
		// Msg pool: [],           Chain: b[m0] -> b[m1] -> b[m2, m3] -> b[m4] -> b[m5, m6]
		store := hamt.NewCborStore()
		p := NewMessagePool()
		m := types.NewMsgs(7)
		MustAdd(p, m[2], m[5])
		oldChain := NewChainWithMessages(store, TipSet{}, msgsSet{msgs{m[0]}}, msgsSet{msgs{m[1]}})
		oldTipSet := headOf(oldChain)
		newChain := NewChainWithMessages(store, oldChain[1],
			msgsSet{msgs{m[2], m[3]}},
			msgsSet{msgs{m[4]}},
			msgsSet{msgs{m[5], m[6]}},
		)
		newTipSet := headOf(newChain)
		UpdateMessagePool(ctx, p, store, oldTipSet, newTipSet)
		assertPoolEquals(assert, p)
	})
}

func TestOrderMessagesByNonce(t *testing.T) {
	t.Run("Empty pool", func(t *testing.T) {
		assert := assert.New(t)
		p := NewMessagePool()
		ordered := OrderMessagesByNonce(p.Pending())
		assert.Equal(0, len(ordered))
	})

	t.Run("Msgs in three orders", func(t *testing.T) {
		assert := assert.New(t)
		p := NewMessagePool()
		m := types.NewMsgs(9)

		// Three in increasing nonce order.
		m[3].From = m[0].From
		m[6].From = m[0].From
		m[0].Nonce = 0
		m[3].Nonce = 1
		m[6].Nonce = 20

		// Three in decreasing nonce order.
		m[4].From = m[1].From
		m[7].From = m[1].From
		m[1].Nonce = 15
		m[4].Nonce = 1
		m[7].Nonce = 0

		// Three out of order.
		m[5].From = m[2].From
		m[8].From = m[2].From
		m[2].Nonce = 5
		m[5].Nonce = 7
		m[8].Nonce = 0

		MustAdd(p, m...)
		ordered := OrderMessagesByNonce(p.Pending())
		assert.Equal(len(p.Pending()), len(ordered))

		lastSeen := make(map[types.Address]uint64)
		for _, m := range ordered {
			last, seen := lastSeen[m.From]
			if seen {
				assert.True(last <= m.Nonce)
			}
			lastSeen[m.From] = m.Nonce
		}
	})
}

func TestLargestNonce(t *testing.T) {
	assert := assert.New(t)

	t.Run("No matches", func(t *testing.T) {
		p := NewMessagePool()
		m := types.NewMsgs(2)
		MustAdd(p, m[0], m[1])
		_, found := LargestNonce(p, types.NewAddressForTestGetter()())
		assert.False(found)
	})

	t.Run("Match, largest is zero", func(t *testing.T) {
		p := NewMessagePool()
		m := types.NewMsgs(1)
		m[0].Nonce = 0
		MustAdd(p, m[0])
		largest, found := LargestNonce(p, m[0].From)
		assert.True(found)
		assert.Equal(uint64(0), largest)
	})

	t.Run("Match", func(t *testing.T) {
		p := NewMessagePool()
		m := types.NewMsgs(3)
		m[1].Nonce = 1
		m[2].Nonce = 2
		m[2].From = m[1].From
		MustAdd(p, m[0], m[1], m[2])
		largest, found := LargestNonce(p, m[2].From)
		assert.True(found)
		assert.Equal(uint64(2), largest)
	})
}
