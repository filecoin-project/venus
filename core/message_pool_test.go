package core

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/types"
)

func TestTxPoolAsync(t *testing.T) {
	assert := assert.New(t)

	count := 400
	msgs := make([]*types.Message, count)

	for i := 0; i < count; i++ {
		msgs[i] = &types.Message{
			To:     types.Address(fmt.Sprintf("Alice-%d", i)),
			From:   types.Address(fmt.Sprintf("Bob-%d", i)),
			Method: "send",
			Params: []interface{}{"1", "2", fmt.Sprintf("%d", i)},
		}
	}

	pool := NewMessagePool()
	var wg sync.WaitGroup

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			for j := 0; j < count/4; j++ {
				err := pool.Add(msgs[j+(count/4)*i])
				assert.NoError(err)
			}
			wg.Done()
		}(i)
	}

	wg.Wait()
	assert.Len(pool.Pending(), count)
}
