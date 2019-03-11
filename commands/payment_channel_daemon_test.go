package commands_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"

	"gx/ipfs/QmekxXDhCxCJRNuzmHreuaT3BsuJcsjcXWNrtV9C8DRHtd/go-multibase"

	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/fixtures"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestPaymentChannelCreateSuccess(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	d := makeTestDaemonWithMinerAndStart(t)
	defer d.ShutdownSuccess()

	args := []string{"paych", "create"}
	args = append(args, "--from", fixtures.TestAddresses[0], "--gas-price", "0", "--gas-limit", "300")
	args = append(args, fixtures.TestAddresses[1], "10000", "20")

	paymentChannelCmd := d.RunSuccess(args...)
	messageCid, err := cid.Parse(strings.Trim(paymentChannelCmd.ReadStdout(), "\n"))
	require.NoError(t, err)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		wait := d.RunSuccess("message", "wait",
			"--return",
			"--message=false",
			"--receipt=false",
			messageCid.String(),
		)
		_, ok := types.NewChannelIDFromString(strings.Trim(wait.ReadStdout(), "\n"), 10)
		assert.True(ok)
		wg.Done()
	}()

	d.RunSuccess("mining once")

	wg.Wait()
}

func TestPaymentChannelLs(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)

	t.Run("Works with default payer", func(t *testing.T) {
		t.Parallel()

		payer, err := address.NewFromString(fixtures.TestAddresses[2])
		require.NoError(err)
		target, err := address.NewFromString(fixtures.TestAddresses[1])
		require.NoError(err)

		eol := types.NewBlockHeight(20)
		amt := types.NewAttoFILFromFIL(10000)

		daemonTestWithPaymentChannel(t, &payer, &target, amt, eol, func(d *th.TestDaemon, channelID *types.ChannelID) {
			ls := listChannelsAsStrs(d, &payer)[0]

			assert.Equal(fmt.Sprintf("%s: target: %s, amt: 10000, amt redeemed: 0, eol: 20", channelID, target.String()), ls)
		})
	})

	t.Run("Works with specified payer", func(t *testing.T) {
		t.Parallel()

		payer, err := address.NewFromString(fixtures.TestAddresses[2])
		require.NoError(err)
		target, err := address.NewFromString(fixtures.TestAddresses[1])
		require.NoError(err)

		eol := types.NewBlockHeight(20)
		amt := types.NewAttoFILFromFIL(10000)

		daemonTestWithPaymentChannel(t, &payer, &target, amt, eol, func(d *th.TestDaemon, channelID *types.ChannelID) {
			args := []string{"paych", "ls"}
			args = append(args, "--from", target.String())
			args = append(args, "--payer", payer.String())

			ls := th.RunSuccessLines(d, args...)[0]

			assert.Equal(fmt.Sprintf("%s: target: %s, amt: 10000, amt redeemed: 0, eol: 20", channelID, target.String()), ls)
		})
	})

	t.Run("Notifies when channels not found", func(t *testing.T) {
		t.Parallel()

		payer, err := address.NewFromString(fixtures.TestAddresses[2])
		require.NoError(err)
		target, err := address.NewFromString(fixtures.TestAddresses[1])
		require.NoError(err)

		eol := types.NewBlockHeight(20)
		amt := types.NewAttoFILFromFIL(10000)

		daemonTestWithPaymentChannel(t, &payer, &target, amt, eol, func(d *th.TestDaemon, channelID *types.ChannelID) {
			ls := listChannelsAsStrs(d, &target)[0]

			assert.Equal("no channels", ls)
		})
	})
}

func TestPaymentChannelVoucherSuccess(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	payer, err := address.NewFromString(fixtures.TestAddresses[2])
	require.NoError(err)
	target, err := address.NewFromString(fixtures.TestAddresses[1])
	require.NoError(err)

	eol := types.NewBlockHeight(20)
	amt := types.NewAttoFILFromFIL(10000)

	daemonTestWithPaymentChannel(t, &payer, &target, amt, eol, func(d *th.TestDaemon, channelID *types.ChannelID) {
		assert := assert.New(t)

		voucher := mustCreateVoucher(t, d, channelID, types.NewAttoFILFromFIL(100), &payer)

		assert.Equal(*types.NewAttoFILFromFIL(100), voucher.Amount)
	})
}

func TestPaymentChannelRedeemSuccess(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	payer, err := address.NewFromString(fixtures.TestAddresses[2])
	require.NoError(err)
	target, err := address.NewFromString(fixtures.TestAddresses[1])
	require.NoError(err)

	eol := types.NewBlockHeight(20)
	amt := types.NewAttoFILFromFIL(10000)

	targetDaemon := th.NewDaemon(
		t,
		// must include 0th keyfilepath if using 0th TestMiner
		th.WithMiner(fixtures.TestMiners[0]),
		th.KeyFile(fixtures.KeyFilePaths()[0]),
		th.KeyFile(fixtures.KeyFilePaths()[1]),
	).Start()
	defer targetDaemon.ShutdownSuccess()

	daemonTestWithPaymentChannel(t, &payer, &target, amt, eol, func(d *th.TestDaemon, channelID *types.ChannelID) {
		assert := assert.New(t)

		d.ConnectSuccess(targetDaemon)

		voucher := createVoucherStr(t, d, channelID, types.NewAttoFILFromFIL(111), &payer, uint64(0))

		mustRedeemVoucher(t, targetDaemon, voucher, &target)

		ls := listChannelsAsStrs(targetDaemon, &payer)[0]
		assert.Equal(fmt.Sprintf("%v: target: %s, amt: 10000, amt redeemed: 111, eol: 20", channelID.String(), target.String()), ls)
	})
}

func TestPaymentChannelRedeemTooEarlyFails(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	payer, err := address.NewFromString(fixtures.TestAddresses[2])
	require.NoError(err)
	target, err := address.NewFromString(fixtures.TestAddresses[1])
	require.NoError(err)

	eol := types.NewBlockHeight(20)
	amt := types.NewAttoFILFromFIL(10000)

	targetDaemon := th.NewDaemon(
		t,
		// must include 0th keyfilepath if using 0th TestMiner
		th.WithMiner(fixtures.TestMiners[0]),
		th.KeyFile(fixtures.KeyFilePaths()[0]),
		th.KeyFile(fixtures.KeyFilePaths()[1]),
	).Start()
	defer targetDaemon.ShutdownSuccess()

	daemonTestWithPaymentChannel(t, &payer, &target, amt, eol, func(d *th.TestDaemon, channelID *types.ChannelID) {
		assert := assert.New(t)

		d.ConnectSuccess(targetDaemon)

		voucher := createVoucherStr(t, d, channelID, types.NewAttoFILFromFIL(111), &payer, uint64(8))

		// Wait for the voucher message to be processed.
		mustRedeemVoucher(t, targetDaemon, voucher, &target)

		ls := listChannelsAsStrs(targetDaemon, &payer)[0]
		assert.Equal(fmt.Sprintf("%v: target: %s, amt: 10000, amt redeemed: 0, eol: 20", channelID.String(), target.String()), ls)
	})
}

func TestPaymentChannelReclaimSuccess(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Initial Balance 10,000
	payer, err := address.NewFromString(fixtures.TestAddresses[2])
	require.NoError(err)
	// Initial Balance 50,000
	target, err := address.NewFromString(fixtures.TestAddresses[1])
	require.NoError(err)

	// Not used in logic
	eol := types.NewBlockHeight(5)
	amt := types.NewAttoFILFromFIL(1000)

	targetDaemon := th.NewDaemon(t,
		th.KeyFile(fixtures.KeyFilePaths()[1]),
		// must include 0th keyfilepath if using 0th TestMiner
		th.KeyFile(fixtures.KeyFilePaths()[0]),
		th.WithMiner(fixtures.TestMiners[0])).Start()
	defer targetDaemon.ShutdownSuccess()

	daemonTestWithPaymentChannel(t, &payer, &target, amt, eol, func(d *th.TestDaemon, channelID *types.ChannelID) {
		assert := assert.New(t)

		d.ConnectSuccess(targetDaemon)

		// payer creates a voucher to be redeemed by target (off-chain)
		voucher := createVoucherStr(t, d, channelID, types.NewAttoFILFromFIL(10), &payer, uint64(0))

		// target redeems the voucher (on-chain)
		mustRedeemVoucher(t, targetDaemon, voucher, &target)

		lsStr := listChannelsAsStrs(targetDaemon, &payer)[0]
		assert.Equal(fmt.Sprintf("%v: target: %s, amt: 1000, amt redeemed: 10, eol: %s", channelID, target.String(), eol.String()), lsStr)

		d.RunSuccess("mining once")
		d.RunSuccess("mining once")

		// payer reclaims channel funds (on-chain)
		mustReclaimChannel(t, d, channelID, &payer)

		lsStr = listChannelsAsStrs(d, &payer)[0]
		assert.Contains(lsStr, "no channels")

		args := []string{"wallet", "balance", payer.String()}
		balStr := th.RunSuccessFirstLine(d, args...)

		// channel's original locked funds minus the redeemed voucher amount
		// are returned to the payer
		assert.Equal("999999999990", balStr)
	})
}

func TestPaymentChannelCloseSuccess(t *testing.T) {
	require := require.New(t)

	// Initial Balance 10,000,000
	payerA, err := address.NewFromString(fixtures.TestAddresses[2])
	require.NoError(err)

	// Initial Balance 10,000,000
	targetA, err := address.NewFromString(fixtures.TestAddresses[1])
	require.NoError(err)
	payer := &payerA
	target := &targetA
	eol := types.NewBlockHeight(100)
	amt := types.NewAttoFILFromFIL(10000)

	targetDaemon := th.NewDaemon(t,
		th.KeyFile(fixtures.KeyFilePaths()[1]),
		// must include 0th keyfilepath if using 0th TestMiner
		th.KeyFile(fixtures.KeyFilePaths()[0]),
		th.WithMiner(fixtures.TestMiners[0])).Start()
	defer targetDaemon.ShutdownSuccess()

	daemonTestWithPaymentChannel(t, payer, target, amt, eol, func(d *th.TestDaemon, channelID *types.ChannelID) {
		assert := assert.New(t)

		d.ConnectSuccess(targetDaemon)

		// payer creates a voucher to be redeemed by target (off-chain)
		voucher := mustCreateVoucher(t, d, channelID, types.NewAttoFILFromFIL(10), payer)

		// target redeems the voucher (on-chain) and simultaneously closes the channel
		mustCloseChannel(t, targetDaemon, voucher, target)

		// channel has been closed
		lsStr := listChannelsAsStrs(targetDaemon, payer)[0]
		assert.Contains(lsStr, "no channels")

		// channel's original locked funds minus the redeemed voucher amount
		// are returned to the payer
		args := []string{"wallet", "balance", payer.String()}
		balStr := th.RunSuccessFirstLine(targetDaemon, args...)
		assert.Equal("999999999990", balStr)

		// target's balance reflects redeemed voucher
		args = []string{"wallet", "balance", target.String()}
		balStr = th.RunSuccessFirstLine(targetDaemon, args...)
		assert.Equal("1000000000010", balStr)
	})
}

func TestPaymentChannelExtendSuccess(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	payer, err := address.NewFromString(fixtures.TestAddresses[2])
	require.NoError(err)
	target, err := address.NewFromString(fixtures.TestAddresses[1])
	require.NoError(err)

	eol := types.NewBlockHeight(5)
	amt := types.NewAttoFILFromFIL(2000)

	daemonTestWithPaymentChannel(t, &payer, &target, amt, eol, func(d *th.TestDaemon, channelID *types.ChannelID) {
		assert := assert.New(t)

		extendedEOL := types.NewBlockHeight(6)
		extendedAmt := types.NewAttoFILFromFIL(3001)

		lsStr := listChannelsAsStrs(d, &payer)[0]
		assert.Equal(fmt.Sprintf("%v: target: %s, amt: 2000, amt redeemed: 0, eol: %s", channelID.String(), target.String(), eol.String()), lsStr)

		mustExtendChannel(t, d, channelID, extendedAmt, extendedEOL, &payer)

		lsStr = listChannelsAsStrs(d, &payer)[0]
		assert.Equal(fmt.Sprintf("%v: target: %s, amt: %s, amt redeemed: 0, eol: %s", channelID.String(), target.String(), extendedAmt.Add(amt), extendedEOL), lsStr)
	})
}

func daemonTestWithPaymentChannel(t *testing.T, payerAddress *address.Address, targetAddress *address.Address,
	fundsToLock *types.AttoFIL, eol *types.BlockHeight, f func(*th.TestDaemon, *types.ChannelID)) {
	assert := assert.New(t)
	require := require.New(t)

	d := th.NewDaemon(
		t,
		// must include 0th keyfilepath with TestMiner 0
		th.WithMiner(fixtures.TestMiners[0]),
		th.KeyFile(fixtures.KeyFilePaths()[0]),
		th.KeyFile(fixtures.KeyFilePaths()[2]),
	).Start()
	defer d.ShutdownSuccess()

	args := []string{"paych", "create"}
	args = append(args, "--from", payerAddress.String(), "--gas-price", "0", "--gas-limit", "300")
	args = append(args, targetAddress.String(), fundsToLock.String(), eol.String())

	paymentChannelCmd := d.RunSuccess(args...)
	messageCid, err := cid.Parse(strings.Trim(paymentChannelCmd.ReadStdout(), "\n"))
	require.NoError(err)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		wait := d.RunSuccess("message", "wait",
			"--return",
			"--message=false",
			"--receipt=false",
			messageCid.String(),
		)
		stdout := strings.Trim(wait.ReadStdout(), "\n")
		channelID, ok := types.NewChannelIDFromString(stdout, 10)
		assert.True(ok)

		f(d, channelID)

		wg.Done()
	}()

	d.RunSuccess("mining once")
	wg.Wait()
}

func mustCreateVoucher(t *testing.T, d *th.TestDaemon, channelID *types.ChannelID, amount *types.AttoFIL, fromAddress *address.Address) paymentbroker.PaymentVoucher {
	require := require.New(t)

	voucherString := createVoucherStr(t, d, channelID, amount, fromAddress, uint64(0))

	_, cborVoucher, err := multibase.Decode(voucherString)
	require.NoError(err)

	var voucher paymentbroker.PaymentVoucher
	err = cbor.DecodeInto(cborVoucher, &voucher)
	require.NoError(err)

	return voucher
}

func createVoucherStr(t *testing.T, d *th.TestDaemon, channelID *types.ChannelID, amount *types.AttoFIL, payerAddress *address.Address, validAt uint64) string {
	args := []string{"paych", "voucher", channelID.String(), amount.String()}
	args = append(args, "--from", payerAddress.String(), "--validat", fmt.Sprintf("%d", validAt))

	return th.RunSuccessFirstLine(d, args...)
}

func listChannelsAsStrs(d *th.TestDaemon, fromAddress *address.Address) []string {
	args := []string{"paych", "ls"}
	args = append(args, "--from", fromAddress.String())

	return th.RunSuccessLines(d, args...)
}

func mustExtendChannel(t *testing.T, d *th.TestDaemon, channelID *types.ChannelID, amount *types.AttoFIL, eol *types.BlockHeight, payerAddress *address.Address) {
	require := require.New(t)

	args := []string{"paych", "extend"}
	args = append(args, "--from", payerAddress.String(), "--gas-price", "0", "--gas-limit", "300")
	args = append(args, channelID.String(), amount.String(), eol.String())

	redeemCmd := d.RunSuccess(args...)
	messageCid, err := cid.Parse(strings.Trim(redeemCmd.ReadStdout(), "\n"))
	require.NoError(err)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		_ = d.RunSuccess("message", "wait",
			"--return=false",
			"--message=false",
			"--receipt=false",
			messageCid.String(),
		)

		wg.Done()
	}()

	d.RunSuccess("mining once")

	wg.Wait()
}

func mustRedeemVoucher(t *testing.T, d *th.TestDaemon, voucher string, targetAddress *address.Address) {
	require := require.New(t)

	args := []string{"paych", "redeem", voucher}
	args = append(args, "--from", targetAddress.String(), "--gas-price", "0", "--gas-limit", "300")

	redeemCmd := d.RunSuccess(args...)
	messageCid, err := cid.Parse(strings.Trim(redeemCmd.ReadStdout(), "\n"))
	require.NoError(err)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		_ = d.RunSuccess("message", "wait",
			"--return=false",
			"--message=false",
			"--receipt=false",
			messageCid.String(),
		)

		wg.Done()
	}()

	d.RunSuccess("mining once")

	wg.Wait()
}

func mustCloseChannel(t *testing.T, d *th.TestDaemon, voucher paymentbroker.PaymentVoucher, targetAddress *address.Address) {
	require := require.New(t)

	args := []string{"paych", "close", mustEncodeVoucherStr(t, voucher)}
	args = append(args, "--from", targetAddress.String(), "--gas-price", "0", "--gas-limit", "300")

	redeemCmd := d.RunSuccess(args...)
	messageCid, err := cid.Parse(strings.Trim(redeemCmd.ReadStdout(), "\n"))
	require.NoError(err)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		_ = d.RunSuccess("message", "wait",
			"--return=false",
			"--message=false",
			"--receipt=false",
			messageCid.String(),
		)

		wg.Done()
	}()

	d.RunSuccess("mining once")

	wg.Wait()
}

func mustReclaimChannel(t *testing.T, d *th.TestDaemon, channelID *types.ChannelID, payerAddress *address.Address) {
	require := require.New(t)

	args := []string{"paych", "reclaim", channelID.String()}
	args = append(args, "--from", payerAddress.String(), "--gas-price", "0", "--gas-limit", "300")

	reclaimCmd := d.RunSuccess(args...)
	messageCid, err := cid.Parse(strings.Trim(reclaimCmd.ReadStdout(), "\n"))
	require.NoError(err)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		_ = d.RunSuccess("message", "wait",
			"--return=false",
			"--message=false",
			"--receipt=true",
			messageCid.String(),
		)
		wg.Done()
	}()

	d.RunSuccess("mining once")

	wg.Wait()
}

func mustEncodeVoucherStr(t *testing.T, voucher paymentbroker.PaymentVoucher) string {
	require := require.New(t)

	bytes, err := cbor.DumpObject(voucher)
	require.NoError(err)

	encoded, err := multibase.Encode(multibase.Base58BTC, bytes)
	require.NoError(err)

	return encoded
}
