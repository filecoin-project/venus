package commands

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	cbor "gx/ipfs/QmSyK1ZiAP98YvnxsTfQpb669V2xeTHRbG4Y6fgKS3vVSd/go-ipld-cbor"
	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gx/ipfs/QmexBtiTTEwwn42Yi6ouKt6VqzpA6wjJgiW1oh9VfaRrup/go-multibase"

	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testfiles"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestPaymentChannelCreateSuccess(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	wtf := tf.WalletFilePath()
	d := NewDaemon(t, WalletFile(wtf), WalletAddr(testAddress1)).Start()
	defer d.ShutdownSuccess()

	args := []string{"paych", "create"}
	args = append(args, "--from", testAddress1)
	args = append(args, testAddress2, "10000", "20")

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

		payer, err := types.NewAddressFromString(testAddress1)
		require.NoError(err)
		target, err := types.NewAddressFromString(testAddress2)
		require.NoError(err)

		eol := types.NewBlockHeight(20)
		amt := types.NewAttoFILFromFIL(10000)

		daemonTestWithPaymentChannel(t, &payer, &target, amt, eol, func(d *TestDaemon, channelID *types.ChannelID) {

			ls := listChannelsAsStrs(d, &payer)[0]

			assert.Equal(fmt.Sprintf("%v: target: %s, amt: 10000, amt redeemed: 0, eol: 20", channelID, target.String()), ls)
		})
	})

	t.Run("Works with specified payer", func(t *testing.T) {
		t.Parallel()

		payer, err := types.NewAddressFromString(testAddress1)
		require.NoError(err)
		target, err := types.NewAddressFromString(testAddress2)
		require.NoError(err)

		eol := types.NewBlockHeight(20)
		amt := types.NewAttoFILFromFIL(10000)

		daemonTestWithPaymentChannel(t, &payer, &target, amt, eol, func(d *TestDaemon, channelID *types.ChannelID) {
			args := []string{"paych", "ls"}
			args = append(args, "--from", target.String())
			args = append(args, "--payer", payer.String())

			ls := runSuccessLines(d, args...)[0]

			assert.Equal(fmt.Sprintf("%v: target: %s, amt: 10000, amt redeemed: 0, eol: 20", channelID, target.String()), ls)
		})
	})

	t.Run("Notifies when channels not found", func(t *testing.T) {
		t.Parallel()

		payer, err := types.NewAddressFromString(testAddress1)
		require.NoError(err)
		target, err := types.NewAddressFromString(testAddress2)
		require.NoError(err)

		eol := types.NewBlockHeight(20)
		amt := types.NewAttoFILFromFIL(10000)

		daemonTestWithPaymentChannel(t, &payer, &target, amt, eol, func(d *TestDaemon, channelID *types.ChannelID) {

			ls := listChannelsAsStrs(d, &target)[0]

			assert.Equal("no channels", ls)
		})
	})
}

func TestPaymentChannelVoucherSuccess(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	payer, err := types.NewAddressFromString(testAddress1)
	require.NoError(err)
	target, err := types.NewAddressFromString(testAddress2)
	require.NoError(err)

	eol := types.NewBlockHeight(20)
	amt := types.NewAttoFILFromFIL(10000)

	daemonTestWithPaymentChannel(t, &payer, &target, amt, eol, func(d *TestDaemon, channelID *types.ChannelID) {
		assert := assert.New(t)

		voucher := mustCreateVoucher(t, d, channelID, types.NewAttoFILFromFIL(100), &payer)

		assert.Equal(*types.NewAttoFILFromFIL(100), voucher.Amount)
	})
}

func TestPaymentChannelRedeemSuccess(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	payer, err := types.NewAddressFromString(testAddress1)
	require.NoError(err)
	target, err := types.NewAddressFromString(testAddress2)
	require.NoError(err)

	eol := types.NewBlockHeight(20)
	amt := types.NewAttoFILFromFIL(10000)

	daemonTestWithPaymentChannel(t, &payer, &target, amt, eol, func(d *TestDaemon, channelID *types.ChannelID) {
		assert := assert.New(t)

		voucher := mustCreateVoucher(t, d, channelID, types.NewAttoFILFromFIL(111), &payer)

		mustRedeemVoucher(t, d, voucher, &target)

		ls := listChannelsAsStrs(d, &payer)[0]
		assert.Equal(fmt.Sprintf("%v: target: %s, amt: 10000, amt redeemed: 111, eol: 20", channelID, target.String()), ls)
	})
}

func TestPaymentChannelReclaimSuccess(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Initial Balance 10,000,000
	payer, err := types.NewAddressFromString(testAddress1)
	require.NoError(err)
	// Initial Balance 10,000,000
	target, err := types.NewAddressFromString(testAddress2)
	require.NoError(err)

	// Not used in logic
	eol := types.NewBlockHeight(20)
	amt := types.NewAttoFILFromFIL(10000)

	daemonTestWithPaymentChannel(t, &payer, &target, amt, eol, func(d *TestDaemon, channelID *types.ChannelID) {
		assert := assert.New(t)

		// payer creates a voucher to be redeemed by target (off-chain)
		voucher := mustCreateVoucher(t, d, channelID, types.NewAttoFILFromFIL(10), &payer)

		// target redeems the voucher (on-chain)
		mustRedeemVoucher(t, d, voucher, &target)

		lsStr := listChannelsAsStrs(d, &payer)[0]
		assert.Equal(fmt.Sprintf("%v: target: %s, amt: 10000, amt redeemed: 10, eol: %s", channelID, target.String(), eol.String()), lsStr)

		d.RunSuccess("mining once")
		d.RunSuccess("mining once")

		// payer reclaims channel funds (on-chain)
		mustReclaimChannel(t, d, channelID, &payer)

		lsStr = listChannelsAsStrs(d, &payer)[0]
		assert.Contains(lsStr, "no channels")

		args := []string{"wallet", "balance", payer.String()}
		balStr := runSuccessFirstLine(d, args...)

		// channel's original locked funds minus the redeemed voucher amount
		// are returned to the payer
		assert.Equal("9999990", balStr)
	})
}

func TestPaymentChannelCloseSuccess(t *testing.T) {
	require := require.New(t)

	// Initial Balance 10,000,000
	payerA, err := types.NewAddressFromString(testAddress1)
	require.NoError(err)
	// Initial Balance 10,000,000
	targetA, err := types.NewAddressFromString(testAddress2)
	require.NoError(err)
	payer := &payerA
	target := &targetA
	eol := types.NewBlockHeight(100)
	amt := types.NewAttoFILFromFIL(10000)

	daemonTestWithPaymentChannel(t, payer, target, amt, eol, func(d *TestDaemon, channelID *types.ChannelID) {
		assert := assert.New(t)

		// payer creates a voucher to be redeemed by target (off-chain)
		voucher := mustCreateVoucher(t, d, channelID, types.NewAttoFILFromFIL(10), payer)

		// target redeems the voucher (on-chain) and simultaneously closes the channel
		mustCloseChannel(t, d, voucher, target)

		// channel has been closed
		lsStr := listChannelsAsStrs(d, payer)[0]
		assert.Contains(lsStr, "no channels")

		// channel's original locked funds minus the redeemed voucher amount
		// are returned to the payer
		args := []string{"wallet", "balance", payer.String()}
		balStr := runSuccessFirstLine(d, args...)
		assert.Equal("9999990", balStr)

		// target's balance reflects redeemed voucher
		args = []string{"wallet", "balance", target.String()}
		balStr = runSuccessFirstLine(d, args...)
		assert.Equal("10000010", balStr)
	})
}

func TestPaymentChannelExtendSuccess(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	payer, err := types.NewAddressFromString(testAddress1)
	require.NoError(err)
	target, err := types.NewAddressFromString(testAddress2)
	require.NoError(err)

	eol := types.NewBlockHeight(5)
	amt := types.NewAttoFILFromFIL(10000)

	daemonTestWithPaymentChannel(t, &payer, &target, amt, eol, func(d *TestDaemon, channelID *types.ChannelID) {
		assert := assert.New(t)

		extendedEOL := types.NewBlockHeight(6)
		extendedAmt := types.NewAttoFILFromFIL(10001)

		lsStr := listChannelsAsStrs(d, &payer)[0]
		assert.Equal(fmt.Sprintf("%v: target: %s, amt: 10000, amt redeemed: 0, eol: %s", channelID, target.String(), eol.String()), lsStr)

		mustExtendChannel(t, d, channelID, extendedAmt, extendedEOL, &payer)

		lsStr = listChannelsAsStrs(d, &payer)[0]
		assert.Equal(fmt.Sprintf("%v: target: %s, amt: %s, amt redeemed: 0, eol: %s", channelID, target.String(), extendedAmt.Add(amt), extendedEOL), lsStr)
	})
}

func daemonTestWithPaymentChannel(t *testing.T, payerAddress *types.Address, targetAddress *types.Address, fundsToLock *types.AttoFIL, eol *types.BlockHeight, f func(*TestDaemon, *types.ChannelID)) {
	assert := assert.New(t)

	wtf := tf.WalletFilePath()
	d := NewDaemon(t, WalletFile(wtf), WalletAddr(payerAddress.String())).Start()
	defer d.ShutdownSuccess()

	args := []string{"paych", "create"}
	args = append(args, "--from", payerAddress.String())
	args = append(args, targetAddress.String(), fundsToLock.String(), eol.String())

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
		channelID, ok := types.NewChannelIDFromString(strings.Trim(wait.ReadStdout(), "\n"), 10)
		assert.True(ok)

		f(d, channelID)

		wg.Done()
	}()

	d.RunSuccess("mining once")
	wg.Wait()
}

func mustCreateVoucher(t *testing.T, d *TestDaemon, channelID *types.ChannelID, amount *types.AttoFIL, fromAddress *types.Address) paymentbroker.PaymentVoucher {
	require := require.New(t)

	voucherString := createVoucherStr(t, d, channelID, amount, fromAddress)

	_, cborVoucher, err := multibase.Decode(voucherString)
	require.NoError(err)

	var voucher paymentbroker.PaymentVoucher
	err = cbor.DecodeInto(cborVoucher, &voucher)
	require.NoError(err)

	return voucher
}

func createVoucherStr(t *testing.T, d *TestDaemon, channelID *types.ChannelID, amount *types.AttoFIL, payerAddress *types.Address) string {
	args := []string{"paych", "voucher", channelID.String(), amount.String()}
	args = append(args, "--from", payerAddress.String())

	return runSuccessFirstLine(d, args...)
}

func listChannelsAsStrs(d *TestDaemon, fromAddress *types.Address) []string {
	args := []string{"paych", "ls"}
	args = append(args, "--from", fromAddress.String())

	return runSuccessLines(d, args...)
}

func mustExtendChannel(t *testing.T, d *TestDaemon, channelID *types.ChannelID, amount *types.AttoFIL, eol *types.BlockHeight, payerAddress *types.Address) {
	require := require.New(t)

	args := []string{"paych", "extend"}
	args = append(args, "--from", payerAddress.String())
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

func mustRedeemVoucher(t *testing.T, d *TestDaemon, voucher paymentbroker.PaymentVoucher, targetAddress *types.Address) {
	require := require.New(t)

	args := []string{"paych", "redeem", mustEncodeVoucherStr(t, voucher)}
	args = append(args, "--from", targetAddress.String())

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

func mustCloseChannel(t *testing.T, d *TestDaemon, voucher paymentbroker.PaymentVoucher, targetAddress *types.Address) {
	require := require.New(t)

	args := []string{"paych", "close", mustEncodeVoucherStr(t, voucher)}
	args = append(args, "--from", targetAddress.String())

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

func mustReclaimChannel(t *testing.T, d *TestDaemon, channelID *types.ChannelID, payerAddress *types.Address) {
	require := require.New(t)

	args := []string{"paych", "reclaim", channelID.String()}
	args = append(args, "--from", payerAddress.String())

	reclaimCmd := d.RunSuccess(args...)
	messageCid, err := cid.Parse(strings.Trim(reclaimCmd.ReadStdout(), "\n"))
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

func mustEncodeVoucherStr(t *testing.T, voucher paymentbroker.PaymentVoucher) string {
	require := require.New(t)

	bytes, err := cbor.DumpObject(voucher)
	require.NoError(err)

	encoded, err := multibase.Encode(multibase.Base58BTC, bytes)
	require.NoError(err)

	return encoded
}
