package commands

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

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

func TestClientAddAsk(t *testing.T) {
	assert := assert.New(t)

	daemon := withDaemon(func() {
		// need an address to mine
		_ = makeAddr(t)

		var wg sync.WaitGroup
		var minerAddr types.Address

		wg.Add(1)
		go func() {
			miner := runSuccess(t, fmt.Sprintf("go-filecoin client create-miner --from %s 1000000", core.TestAccount))
			addr, err := types.ParseAddress(strings.Trim(miner.ReadStdout(), "\n"))
			assert.NoError(err)
			assert.NotEqual(addr, types.Address(""))
			minerAddr = addr
			wg.Done()
		}()

		time.Sleep(100 * time.Millisecond)
		_ = runSuccess(t, "go-filecoin miner gen-block")

		wg.Wait()

		wg.Add(1)
		go func() {
			ask := runSuccess(t, fmt.Sprintf("go-filecoin client add-ask %s 2000 10 --from %s", minerAddr, core.TestAccount))
			askCid, err := cid.Parse(strings.Trim(ask.ReadStdout(), "\n"))
			assert.NoError(err)
			assert.NotNil(askCid)
			wg.Done()
		}()

		time.Sleep(100 * time.Millisecond)
		_ = runSuccess(t, "go-filecoin miner gen-block")

		wg.Wait()
	})
	assert.NoError(daemon.Error)
	assert.Equal(daemon.Code, 0)
}
