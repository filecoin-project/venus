package commands

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestClientCreateMiner(t *testing.T) {
	assert := assert.New(t)

	daemon := withDaemon(func() {
		// need an address to mine
		_ = makeAddr(t)

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			miner := runSuccess(t, fmt.Sprintf("go-filecoin client create-miner --from %s 1000000", core.TestAccount))
			addr, err := types.ParseAddress(strings.Trim(miner.ReadStdout(), "\n"))
			assert.NoError(err)
			assert.NotEqual(addr, types.Address(""))
			wg.Done()
		}()

		time.Sleep(100 * time.Millisecond)
		_ = runSuccess(t, "go-filecoin miner gen-block")

		wg.Wait()
	})
	assert.NoError(daemon.Error)
	assert.Equal(daemon.Code, 0)
}
