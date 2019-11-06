package commands_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/fixtures"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestChainHead(t *testing.T) {
	tf.IntegrationTest(t)

	d := th.NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	jsonResult := d.RunSuccess("chain", "head", "--enc", "json").ReadStdoutTrimNewlines()

	var cidsFromJSON []cid.Cid
	err := json.Unmarshal([]byte(jsonResult), &cidsFromJSON)
	assert.NoError(t, err)

	textResult := d.RunSuccess("chain", "ls", "--enc", "text").ReadStdoutTrimNewlines()

	textCid, err := cid.Decode(textResult)
	require.NoError(t, err)

	assert.Equal(t, textCid, cidsFromJSON[0])
}

func TestChainLs(t *testing.T) {
	tf.IntegrationTest(t)

	t.Run("chain ls with json encoding returns the whole chain as json", func(t *testing.T) {
		d := makeTestDaemonWithMinerAndStart(t)
		defer d.ShutdownSuccess()
		op1 := d.RunSuccess("mining", "once", "--enc", "text")
		result1 := op1.ReadStdoutTrimNewlines()
		c, err := cid.Parse(result1)
		require.NoError(t, err)

		op2 := d.RunSuccess("chain", "ls", "--enc", "json")
		result2 := op2.ReadStdoutTrimNewlines()

		var bs [][]block.Block
		for _, line := range bytes.Split([]byte(result2), []byte{'\n'}) {
			var b []block.Block
			err := json.Unmarshal(line, &b)
			require.NoError(t, err)
			bs = append(bs, b)
			require.Equal(t, 1, len(b))
		}

		assert.Equal(t, 2, len(bs))
		assert.True(t, bs[1][0].Parents.Empty())
		assert.True(t, c.Equals(bs[0][0].Cid()))
	})

	t.Run("chain ls with chain of size 1 returns genesis block", func(t *testing.T) {
		d := th.NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		op := d.RunSuccess("chain", "ls", "--enc", "json")
		result := op.ReadStdoutTrimNewlines()

		var b []block.Block
		err := json.Unmarshal([]byte(result), &b)
		require.NoError(t, err)

		assert.True(t, b[0].Parents.Empty())
	})

	t.Run("chain ls with text encoding returns only CIDs", func(t *testing.T) {
		daemon := makeTestDaemonWithMinerAndStart(t)
		defer daemon.ShutdownSuccess()

		var blocks []block.Block
		blockJSON := daemon.RunSuccess("chain", "ls", "--enc", "json").ReadStdoutTrimNewlines()
		err := json.Unmarshal([]byte(blockJSON), &blocks)
		genesisBlockCid := blocks[0].Cid().String()
		require.NoError(t, err)

		newBlockCid := daemon.RunSuccess("mining", "once", "--enc", "text").ReadStdoutTrimNewlines()

		expectedOutput := fmt.Sprintf("%s\n%s", newBlockCid, genesisBlockCid)

		chainLsResult := daemon.RunSuccess("chain", "ls").ReadStdoutTrimNewlines()

		assert.Equal(t, chainLsResult, expectedOutput)
	})

	t.Run("chain ls --long returns CIDs, Miner, block height and message count", func(t *testing.T) {
		daemon := makeTestDaemonWithMinerAndStart(t)
		defer daemon.ShutdownSuccess()

		newBlockCid := daemon.RunSuccess("mining", "once", "--enc", "text").ReadStdoutTrimNewlines()

		chainLsResult := daemon.RunSuccess("chain", "ls", "--long").ReadStdoutTrimNewlines()

		assert.Contains(t, chainLsResult, newBlockCid)
		assert.Contains(t, chainLsResult, fixtures.TestMiners[0])
		assert.Contains(t, chainLsResult, "1")
		assert.Contains(t, chainLsResult, "0")
	})

	t.Run("chain ls --long with JSON encoding returns integer string block height", func(t *testing.T) {
		daemon := makeTestDaemonWithMinerAndStart(t)
		defer daemon.ShutdownSuccess()

		daemon.RunSuccess("mining", "once", "--enc", "text")
		chainLsResult := daemon.RunSuccess("chain", "ls", "--long", "--enc", "json").ReadStdoutTrimNewlines()
		assert.Contains(t, chainLsResult, `"height":"0"`)
		assert.Contains(t, chainLsResult, `"height":"1"`)
	})
}
