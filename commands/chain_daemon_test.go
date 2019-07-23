package commands_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/fixtures"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
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

		var bs [][]types.Block
		for _, line := range bytes.Split([]byte(result2), []byte{'\n'}) {
			var b []types.Block
			err := json.Unmarshal(line, &b)
			require.NoError(t, err)
			bs = append(bs, b)
			require.Equal(t, 1, len(b))
			line = bytes.TrimPrefix(line, []byte{'['})
			line = bytes.TrimSuffix(line, []byte{']'})

			// ensure conformance with JSON schema
			requireSchemaConformance(t, line, "filecoin_block")
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

		var b []types.Block
		err := json.Unmarshal([]byte(result), &b)
		require.NoError(t, err)

		assert.True(t, b[0].Parents.Empty())
	})

	t.Run("chain ls with text encoding returns only CIDs", func(t *testing.T) {
		daemon := makeTestDaemonWithMinerAndStart(t)
		defer daemon.ShutdownSuccess()

		var blocks []types.Block
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

	t.Run("chain ls --long with JSON encoding returns integer string block height and nonce", func(t *testing.T) {
		daemon := makeTestDaemonWithMinerAndStart(t)
		defer daemon.ShutdownSuccess()

		daemon.RunSuccess("mining", "once", "--enc", "text")
		chainLsResult := daemon.RunSuccess("chain", "ls", "--long", "--enc", "json").ReadStdoutTrimNewlines()
		assert.Contains(t, chainLsResult, `"height":"0"`)
		assert.Contains(t, chainLsResult, `"height":"1"`)
		assert.Contains(t, chainLsResult, `"nonce":"0"`)
	})

	t.Run("chain ls --begin --end with height range", func(t *testing.T) {
		daemon := makeTestDaemonWithMinerAndStart(t)
		defer daemon.ShutdownSuccess()

		daemon.RunSuccess("mining", "once", "--enc", "text")
		daemon.RunSuccess("mining", "once", "--enc", "text")
		daemon.RunSuccess("mining", "once", "--enc", "text")
		daemon.RunSuccess("mining", "once", "--enc", "text")
		chainLsResult := daemon.RunSuccess("chain", "ls", "--long", "-b", "1", "-e", "3", "--enc", "json").ReadStdoutTrimNewlines()
		assert.Contains(t, chainLsResult, `"height":"1"`)
		assert.Contains(t, chainLsResult, `"height":"2"`)
		assert.Contains(t, chainLsResult, `"height":"3"`)
		assert.NotContains(t, chainLsResult, `"height":"0"`)
		assert.NotContains(t, chainLsResult, `"height":"4"`)
		assert.Contains(t, chainLsResult, `"nonce":"0"`)
	})
}

func TestBlockDaemon(t *testing.T) {
	tf.IntegrationTest(t)

	t.Run("chain block <cid-of-genesis-block> returns human readable output for the filecoin block", func(t *testing.T) {
		d := makeTestDaemonWithMinerAndStart(t)
		defer d.ShutdownSuccess()

		// mine a block and get its CID
		minedBlockCidStr := th.RunSuccessFirstLine(d, "mining", "once")

		// get the mined block by its CID
		output := d.RunSuccess("chain", "block", minedBlockCidStr).ReadStdoutTrimNewlines()

		assert.Contains(t, output, "Block Details")
		assert.Contains(t, output, "Weight: 0")
		assert.Contains(t, output, "Height: 1")
		assert.Contains(t, output, "Nonce:  0")
		assert.Contains(t, output, "Timestamp:  ")
	})

	t.Run("chain block --messages <cid-of-genesis-block> returns human readable output for the filecoin block including messages", func(t *testing.T) {
		d := makeTestDaemonWithMinerAndStart(t)
		defer d.ShutdownSuccess()

		// mine a block and get its CID
		minedBlockCidStr := th.RunSuccessFirstLine(d, "mining", "once")

		// get the mined block by its CID
		output := d.RunSuccess("chain", "block", "--messages", minedBlockCidStr).ReadStdoutTrimNewlines()

		assert.Contains(t, output, "Block Details")
		assert.Contains(t, output, "Weight: 0")
		assert.Contains(t, output, "Height: 1")
		assert.Contains(t, output, "Nonce:  0")
		assert.Contains(t, output, "Timestamp:  ")
		assert.Contains(t, output, "Messages:  ")
	})

	t.Run("chain block <cid-of-genesis-block> --enc json returns JSON for a filecoin block", func(t *testing.T) {
		d := th.NewDaemon(t,
			th.KeyFile(fixtures.KeyFilePaths()[0]),
			th.WithMiner(fixtures.TestMiners[0])).Start()
		defer d.ShutdownSuccess()

		// mine a block and get its CID
		minedBlockCidStr := th.RunSuccessFirstLine(d, "mining", "once")

		// get the mined block by its CID
		blockGetLine := th.RunSuccessFirstLine(d, "chain", "block", minedBlockCidStr, "--enc", "json")
		var blockGetBlock types.Block
		require.NoError(t, json.Unmarshal([]byte(blockGetLine), &blockGetBlock))

		// ensure that we were returned the correct block

		require.Equal(t, minedBlockCidStr, blockGetBlock.Cid().String())

		// ensure that the JSON we received from block get conforms to schema

		requireSchemaConformance(t, []byte(blockGetLine), "filecoin_block")
	})
}
