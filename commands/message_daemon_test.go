package commands

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/fixtures"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestMessageSend(t *testing.T) {
	t.Parallel()

	d := th.NewDaemon(
		t,
		th.WithMiner(fixtures.TestMiners[0]),
		th.KeyFile(fixtures.KeyFilePaths()[0]),
		th.KeyFile(fixtures.KeyFilePaths()[1]),
	).Start()
	defer d.ShutdownSuccess()

	d.RunSuccess("mining", "once")

	t.Log("[failure] invalid target")
	d.RunFail(
		"invalid checksum",
		"message", "send",
		"--from", fixtures.TestAddresses[0],
		"--price", "0", "--limit", "300",
		"--value=10", "xyz",
	)

	t.Log("[success] with from")
	defaultaddr := d.GetDefaultAddress()
	d.RunSuccess("message", "send",
		"--from", fixtures.TestAddresses[0],
		"--price", "0", "--limit", "300",
		defaultaddr,
	)

	t.Log("[success] with from and value")
	d.RunSuccess("message", "send",
		"--from", fixtures.TestAddresses[0],
		"--price", "0", "--limit", "300",
		"--value=10", fixtures.TestAddresses[1],
	)
}

func TestMessageWait(t *testing.T) {
	t.Parallel()

	d := th.NewDaemon(
		t,
		th.WithMiner(fixtures.TestMiners[0]),
		th.KeyFile(fixtures.KeyFilePaths()[0]),
	).Start()
	defer d.ShutdownSuccess()

	t.Run("[success] transfer only", func(t *testing.T) {
		assert := assert.New(t)

		msg := d.RunSuccess(
			"message", "send",
			"--from", fixtures.TestAddresses[0],
			"--price", "0", "--limit", "300",
			"--value=10",
			fixtures.TestAddresses[1],
		)

		msgcid := strings.Trim(msg.ReadStdout(), "\n")

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			wait := d.RunSuccess(
				"message", "wait",
				"--message=false",
				"--receipt=false",
				"--return",
				msgcid,
			)
			// nothing should be printed, as there is no return value
			assert.Equal("", wait.ReadStdout())
			wg.Done()
		}()

		d.RunSuccess("mining once")

		wg.Wait()
	})
}

func TestMessageSendBlockGasLimit(t *testing.T) {
	t.Parallel()

	d := th.NewDaemon(
		t,
		th.DefaultAddress(fixtures.TestAddresses[0]),
		th.WithMiner(fixtures.TestMiners[0]),
		th.KeyFile(fixtures.KeyFilePaths()[0]),
	).Start()
	defer d.ShutdownSuccess()

	doubleTheBlockGasLimit := strconv.Itoa(int(types.BlockGasLimit) * 2)
	halfTheBlockGasLimit := strconv.Itoa(int(types.BlockGasLimit) / 2)
	result := struct{ Messages []interface{} }{}

	t.Run("when the gas limit is above the block limit, the message fails", func(t *testing.T) {
		d.RunSuccess(
			"message", "send",
			"--price", "0", "--limit", doubleTheBlockGasLimit,
			"--value=10", fixtures.TestAddresses[1],
		)

		blockCid := d.RunSuccess("mining", "once").ReadStdoutTrimNewlines()

		blockInfo := d.RunSuccess("show", "block", blockCid, "--enc", "json").ReadStdoutTrimNewlines()

		require.NoError(t, json.Unmarshal([]byte(blockInfo), &result))
		assert.Empty(t, result.Messages, "msg over the block gas limit fails validation and is _NOT_ run in the block")
	})

	t.Run("when the gas limit is below the block limit, the message succeeds", func(t *testing.T) {
		d.RunSuccess(
			"message", "send",
			"--price", "0", "--limit", halfTheBlockGasLimit,
			"--value=10", fixtures.TestAddresses[1],
		)

		blockCid := d.RunSuccess("mining", "once").ReadStdoutTrimNewlines()

		blockInfo := d.RunSuccess("show", "block", blockCid, "--enc", "json").ReadStdoutTrimNewlines()

		require.NoError(t, json.Unmarshal([]byte(blockInfo), &result))
		assert.NotEmpty(t, result.Messages, "msg under the block gas limit passes validation and is run in the block")
	})
}
