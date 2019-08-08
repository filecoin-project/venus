package commands_test

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/fixtures"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestMessageSend(t *testing.T) {
	tf.IntegrationTest(t)

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
		"--gas-price", "1",
		"--gas-limit", "300",
		fixtures.TestAddresses[3],
	)

	t.Log("[success] with from and int value")
	d.RunSuccess("message", "send",
		"--from", from,
		"--gas-price", "1",
		"--gas-limit", "300",
		"--value", "10",
		fixtures.TestAddresses[3],
	)

	t.Log("[success] with from and decimal value")
	d.RunSuccess("message", "send",
		"--from", from,
		"--gas-price", "1",
		"--gas-limit", "300",
		"--value", "5.5",
		fixtures.TestAddresses[3],
	)
}

func TestMessageWait(t *testing.T) {
	tf.IntegrationTest(t)

	d := makeTestDaemonWithMinerAndStart(t)
	defer d.ShutdownSuccess()

	t.Run("[success] transfer only", func(t *testing.T) {
		msg := d.RunSuccess("message", "send",
			"--from", fixtures.TestAddresses[0],
			"--gas-price", "1",
			"--gas-limit", "300",
			fixtures.TestAddresses[1],
		)
		msgcid := msg.ReadStdoutTrimNewlines()

		// Fail with timeout before the message has been mined
		d.RunFail(
			"deadline exceeded",
			"message", "wait",
			"--message=false",
			"--receipt=false",
			"--timeout=100ms",
			"--return",
			msgcid,
		)

		d.RunSuccess("mining once")

		wait := d.RunSuccess(
			"message", "wait",
			"--message=false",
			"--receipt=false",
			"--timeout=1m",
			"--return",
			msgcid,
		)
		assert.Equal(t, "", wait.ReadStdout())
	})
}

func TestMessageSendBlockGasLimit(t *testing.T) {
	tf.IntegrationTest(t)

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
	result := struct{ Messages cid.Cid }{}

	t.Run("when the gas limit is above the block limit, the message fails", func(t *testing.T) {
		d.RunFail("block gas limit",
			"message", "send",
			"--gas-price", "1", "--gas-limit", doubleTheBlockGasLimit,
			"--value=10", fixtures.TestAddresses[1],
		)
	})

	t.Run("when the gas limit is below the block limit, the message succeeds", func(t *testing.T) {
		d.RunSuccess(
			"message", "send",
			"--gas-price", "1", "--gas-limit", halfTheBlockGasLimit,
			"--value=10", fixtures.TestAddresses[1],
		)

		blockCid := d.RunSuccess("mining", "once").ReadStdoutTrimNewlines()

		blockInfo := d.RunSuccess("show", "header", blockCid, "--enc", "json").ReadStdoutTrimNewlines()

		require.NoError(t, json.Unmarshal([]byte(blockInfo), &result))
		assert.NotEmpty(t, result.Messages, "msg under the block gas limit passes validation and is run in the block")
	})
}

func TestMessageStatus(t *testing.T) {
	tf.IntegrationTest(t)

	d := makeTestDaemonWithMinerAndStart(t)
	defer d.ShutdownSuccess()

	t.Run("queue then on chain", func(t *testing.T) {
		msg := d.RunSuccess(
			"message", "send",
			"--from", fixtures.TestAddresses[0],
			"--gas-price", "1", "--gas-limit", "300",
			"--value=1234",
			fixtures.TestAddresses[1],
		)

		msgcid := strings.Trim(msg.ReadStdout(), "\n")
		status := d.RunSuccess("message", "status", msgcid).ReadStdout()

		assert.Contains(t, status, "In outbox")
		assert.Contains(t, status, "In mpool")
		assert.NotContains(t, status, "On chain") // not found on chain (yet)
		assert.Contains(t, status, "1234")        // the "value"

		d.RunSuccess("mining once")

		status = d.RunSuccess("message", "status", msgcid).ReadStdout()

		assert.NotContains(t, status, "In outbox")
		assert.NotContains(t, status, "In mpool")
		assert.Contains(t, status, "On chain")
		assert.Contains(t, status, "1234") // the "value"

		status = d.RunSuccess("message", "status", "QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS").ReadStdout()
		assert.NotContains(t, status, "In outbox")
		assert.NotContains(t, status, "In mpool")
		assert.NotContains(t, status, "On chain")
	})
}
