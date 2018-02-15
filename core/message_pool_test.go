package core

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/types"
)

func TestMessagePoolAddRemove(t *testing.T) {
	assert := assert.New(t)

	pool := NewMessagePool()
	msg1 := types.NewMessage(
		types.Address("Alice"),
		types.Address("Bob"),
		nil,
		"balance",
		nil,
	)
	msg2 := types.NewMessage(
		types.Address("Alice"),
		types.Address("Bob"),
		nil,
		"balance",
		[]interface{}{"hello"},
	)

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
	msg1 := types.NewMessage(
		types.Address("Alice"),
		types.Address("Bob"),
		nil,
		"balance",
		nil,
	)

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

	for i := 0; i < count; i++ {
		msgs[i] = types.NewMessage(
			types.Address(fmt.Sprintf("Alice-%d", i)),
			types.Address(fmt.Sprintf("Bob-%d", i)),
			nil,
			"send",
			[]interface{}{"1", "2", fmt.Sprintf("%d", i)},
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
