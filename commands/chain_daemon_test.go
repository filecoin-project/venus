package commands_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/fixtures"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

func TestChainDaemon(t *testing.T) {
	t.Parallel()
	t.Run("chain ls with json encoding returns the whole chain as json", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		require := require.New(t)

		d := makeTestDaemonWithMinerAndStart(t)
		defer d.ShutdownSuccess()

		op1 := d.RunSuccess("mining", "once", "--enc", "text")
		result1 := op1.ReadStdoutTrimNewlines()
		c, err := cid.Parse(result1)
		require.NoError(err)

		op2 := d.RunSuccess("chain", "ls", "--enc", "json")
		result2 := op2.ReadStdoutTrimNewlines()

		var bs [][]types.Block
		for _, line := range bytes.Split([]byte(result2), []byte{'\n'}) {
			var b []types.Block
			err := json.Unmarshal(line, &b)
			require.NoError(err)
			bs = append(bs, b)
			require.Equal(1, len(b))
			line = bytes.TrimPrefix(line, []byte{'['})
			line = bytes.TrimSuffix(line, []byte{']'})

			// ensure conformance with JSON schema
			requireSchemaConformance(t, line, "filecoin_block")
		}

		assert.Equal(2, len(bs))
		assert.True(bs[1][0].Parents.Empty())
		assert.True(c.Equals(bs[0][0].Cid()))
	})

	t.Run("chain head with chain of size 1 returns genesis block", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		require := require.New(t)

		d := th.NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		op := d.RunSuccess("chain", "ls", "--enc", "json")
		result := op.ReadStdoutTrimNewlines()

		var b []types.Block
		err := json.Unmarshal([]byte(result), &b)
		require.NoError(err)

		assert.True(b[0].Parents.Empty())
	})

	t.Run("chain ls with text encoding returns only CIDs", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		require := require.New(t)

		daemon := makeTestDaemonWithMinerAndStart(t)
		defer daemon.ShutdownSuccess()

		var blocks []types.Block
		blockJSON := daemon.RunSuccess("chain", "ls", "--enc", "json").ReadStdoutTrimNewlines()
		err := json.Unmarshal([]byte(blockJSON), &blocks)
		genesisBlockCid := blocks[0].Cid().String()
		require.NoError(err)

		newBlockCid := daemon.RunSuccess("mining", "once", "--enc", "text").ReadStdoutTrimNewlines()

		expectedOutput := fmt.Sprintf("%s\n%s", newBlockCid, genesisBlockCid)

		chainLsResult := daemon.RunSuccess("chain", "ls").ReadStdoutTrimNewlines()

		assert.Equal(chainLsResult, expectedOutput)
	})

	t.Run("chain ls --long returns CIDs, Miner, block height and message count", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		daemon := makeTestDaemonWithMinerAndStart(t)
		defer daemon.ShutdownSuccess()

		newBlockCid := daemon.RunSuccess("mining", "once", "--enc", "text").ReadStdoutTrimNewlines()

		chainLsResult := daemon.RunSuccess("chain", "ls", "--long").ReadStdoutTrimNewlines()

		assert.Contains(chainLsResult, newBlockCid)
		assert.Contains(chainLsResult, fixtures.TestMiners[0])
		assert.Contains(chainLsResult, "1")
		assert.Contains(chainLsResult, "0")
	})

	t.Run("chain ls --long with JSON encoding returns integer string block height and nonce", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		daemon := makeTestDaemonWithMinerAndStart(t)
		defer daemon.ShutdownSuccess()

		daemon.RunSuccess("mining", "once", "--enc", "text")
		chainLsResult := daemon.RunSuccess("chain", "ls", "--long", "--enc", "json").ReadStdoutTrimNewlines()
		assert.Contains(chainLsResult, `"height":"0"`)
		assert.Contains(chainLsResult, `"height":"1"`)
		assert.Contains(chainLsResult, `"nonce":"0"`)
	})
}
