package commands_test

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"testing"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/fixtures"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestMessageSend(t *testing.T) {
	t.Parallel()

	d := th.NewDaemon(
		t,
		th.DefaultAddress(fixtures.TestAddresses[0]),
		th.KeyFile(fixtures.KeyFilePaths()[1]),
		// must include same-index KeyFilePath when configuring with a TestMiner.
		th.WithMiner(fixtures.TestMiners[0]),
		th.KeyFile(fixtures.KeyFilePaths()[0]),
	).Start()
	defer d.ShutdownSuccess()

	d.RunSuccess("mining", "once")

	from := d.GetDefaultAddress() // this should = fixtures.TestAddresses[0]

	t.Log("[failure] invalid target")
	d.RunFail(
		address.ErrUnknownNetwork.Error(),
		"message", "send",
		"--from", from,
		"--gas-price", "0", "--gas-limit", "300",
		"--value=10", "xyz",
	)

	t.Log("[success] with from")
	d.RunSuccess("message", "send",
		"--from", from,
		"--gas-price", "0",
		"--gas-limit", "300",
		fixtures.TestAddresses[3],
	)

	t.Log("[success] with from and value")
	d.RunSuccess("message", "send",
		"--from", from,
		"--gas-price", "0",
		"--gas-limit", "300",
		"--value=10",
		fixtures.TestAddresses[3],
	)
}

func TestMessageWait(t *testing.T) {
	t.Parallel()
	d := makeTestDaemonWithMinerAndStart(t)
	defer d.ShutdownSuccess()

	t.Run("[success] transfer only", func(t *testing.T) {
		assert := assert.New(t)

		msg := d.RunSuccess(
			"message", "send",
			"--from", fixtures.TestAddresses[0],
			"--gas-price", "0", "--gas-limit", "300",
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
		// default address required
		th.DefaultAddress(fixtures.TestAddresses[0]),
		th.WithMiner(fixtures.TestMiners[0]),
		th.KeyFile(fixtures.KeyFilePaths()[0]),
	).Start()
	defer d.ShutdownSuccess()

	doubleTheBlockGasLimit := strconv.Itoa(int(types.BlockGasLimit) * 2)
	halfTheBlockGasLimit := strconv.Itoa(int(types.BlockGasLimit) / 2)
	result := struct{ Messages []interface{} }{}

	t.Run("when the gas limit is above the block limit, the message fails", func(t *testing.T) {
		d.RunFail("block gas limit",
			"message", "send",
			"--gas-price", "0", "--gas-limit", doubleTheBlockGasLimit,
			"--value=10", fixtures.TestAddresses[1],
		)
	})

	t.Run("when the gas limit is below the block limit, the message succeeds", func(t *testing.T) {
		d.RunSuccess(
			"message", "send",
			"--gas-price", "0", "--gas-limit", halfTheBlockGasLimit,
			"--value=10", fixtures.TestAddresses[1],
		)

		blockCid := d.RunSuccess("mining", "once").ReadStdoutTrimNewlines()

		blockInfo := d.RunSuccess("show", "block", blockCid, "--enc", "json").ReadStdoutTrimNewlines()

		require.NoError(t, json.Unmarshal([]byte(blockInfo), &result))
		assert.NotEmpty(t, result.Messages, "msg under the block gas limit passes validation and is run in the block")
	})
}
