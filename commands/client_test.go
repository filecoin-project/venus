package commands

import (
	"fmt"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/core"
)

func TestClientAddBidSuccess(t *testing.T) {
	assert := assert.New(t)

	daemon := withDaemon(func() {
		_ = makeAddr(t)

		bid := runSuccess(t, fmt.Sprintf("go-filecoin client add-bid 2000 10 --from %s", core.TestAccount))
		bidMessageCid, err := cid.Parse(strings.Trim(bid.ReadStdout(), "\n"))
		assert.NoError(err)

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			wait := runSuccess(t, fmt.Sprintf("go-filecoin message wait --return --message=false --receipt=false %s", bidMessageCid.String()))
			out := wait.ReadStdout()
			bidID, ok := new(big.Int).SetString(strings.Trim(out, "\n"), 10)
			assert.True(ok)
			assert.NotNil(bidID)
			wg.Done()
		}()

		time.Sleep(100 * time.Millisecond)
		_ = runSuccess(t, "go-filecoin mining once")

		wg.Wait()
	})
	assert.NoError(daemon.Error)
	assert.Equal(daemon.Code, 0)
}

func TestClientAddBidFail(t *testing.T) {
	assert := assert.New(t)

	daemon := withDaemon(func() {
		// need an address to mine
		_ = makeAddr(t)

		_ = runFail(
			t,
			"invalid from address",
			"go-filecoin client add-bid 2000 10 --from hello",
		)
		_ = runFail(
			t,
			"invalid size",
			fmt.Sprintf("go-filecoin client add-bid 2f 10 --from %s", core.TestAccount),
		)
		_ = runFail(
			t,
			"invalid price",
			fmt.Sprintf("go-filecoin client add-bid 10 3f --from %s", core.TestAccount),
		)
	})
	assert.NoError(daemon.Error)
	assert.Equal(daemon.Code, 0)
}
