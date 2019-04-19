package commands_test

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/fastesting"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestPaymentChannelCreateSuccess(t *testing.T) {
	tf.IntegrationTest(t)

	require := require.New(t)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
	defer cancel()

	// Get basic testing environment
	ctx, env := fastesting.NewTestEnvironment(ctx, t, fast.EnvironmentOpts{})

	// Teardown after test ends
	defer func() {
		err := env.Teardown(ctx)
		require.NoError(err)
	}()

	// Start test
	rsrc := requireNewPaychResource(ctx, t, env)

	channelExpiry := types.NewBlockHeight(20)
	channelAmount := types.NewAttoFILFromFIL(1000)

	rsrc.requirePaymentChannel(ctx, channelAmount, channelExpiry)
}

func TestPaymentChannelLs(t *testing.T) {
	tf.IntegrationTest(t)

	t.Run("Works with default payer", func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
		defer cancel()

		// Get basic testing environment
		ctx, env := fastesting.NewTestEnvironment(ctx, t, fast.EnvironmentOpts{})

		// Teardown after test ends
		defer func() {
			err := env.Teardown(ctx)
			require.NoError(err)
		}()

		// Start test
		rsrc := requireNewPaychResource(ctx, t, env)

		channelExpiry := types.NewBlockHeight(20)
		channelAmount := types.NewAttoFILFromFIL(1000)

		chanid, _ := rsrc.requirePaymentChannel(ctx, channelAmount, channelExpiry)

		channels, err := rsrc.payer.PaychLs(ctx)
		require.NoError(err)

		assert.Len(channels, 1)

		channel := channels[chanid.String()]
		assert.Equal(channelAmount, channel.Amount)
		assert.Equal(channelExpiry, channel.Eol)
		assert.Equal(rsrc.targetAddr, channel.Target)
		assert.Equal(types.ZeroAttoFIL, channel.AmountRedeemed)
	})

	t.Run("Works with specified payer", func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
		defer cancel()

		// Get basic testing environment
		ctx, env := fastesting.NewTestEnvironment(ctx, t, fast.EnvironmentOpts{})

		// Teardown after test ends
		defer func() {
			err := env.Teardown(ctx)
			require.NoError(err)
		}()

		// Start test
		rsrc := requireNewPaychResource(ctx, t, env)

		channelExpiry := types.NewBlockHeight(20)
		channelAmount := types.NewAttoFILFromFIL(1000)

		chanid, _ := rsrc.requirePaymentChannel(ctx, channelAmount, channelExpiry)

		channels, err := rsrc.payer.PaychLs(ctx, fast.AOPayer(rsrc.payerAddr))
		require.NoError(err)

		assert.Len(channels, 1)

		channel := channels[chanid.String()]
		assert.Equal(channelAmount, channel.Amount)
		assert.Equal(channelExpiry, channel.Eol)
		assert.Equal(rsrc.targetAddr, channel.Target)
		assert.Equal(types.ZeroAttoFIL, channel.AmountRedeemed)
	})

	t.Run("No results when listing with different from address", func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
		defer cancel()

		// Get basic testing environment
		ctx, env := fastesting.NewTestEnvironment(ctx, t, fast.EnvironmentOpts{})

		// Teardown after test ends
		defer func() {
			err := env.Teardown(ctx)
			require.NoError(err)
		}()

		// Start test
		rsrc := requireNewPaychResource(ctx, t, env)

		channelExpiry := types.NewBlockHeight(20)
		channelAmount := types.NewAttoFILFromFIL(1000)

		// requirePaymentChannel sets up a channel with the rsrc.payerAddr as the from address
		rsrc.requirePaymentChannel(ctx, channelAmount, channelExpiry)

		channels, err := rsrc.payer.PaychLs(ctx, fast.AOFromAddr(rsrc.targetAddr))
		require.NoError(err)

		assert.Len(channels, 0)
	})
}

func TestPaymentChannelVoucherSuccess(t *testing.T) {
	tf.IntegrationTest(t)

	require := require.New(t)
	assert := assert.New(t)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
	defer cancel()

	// Get basic testing environment
	ctx, env := fastesting.NewTestEnvironment(ctx, t, fast.EnvironmentOpts{})

	// Teardown after test ends
	defer func() {
		err := env.Teardown(ctx)
		require.NoError(err)
	}()

	// Start test
	rsrc := requireNewPaychResource(ctx, t, env)

	channelExpiry := types.NewBlockHeight(20)
	channelAmount := types.NewAttoFILFromFIL(1000)

	chanid, _ := rsrc.requirePaymentChannel(ctx, channelAmount, channelExpiry)

	voucherAmount := types.NewAttoFILFromFIL(10)
	voucherValidAt := types.NewBlockHeight(0)
	voucherStr, err := rsrc.payer.PaychVoucher(ctx, chanid, voucherAmount, fast.AOFromAddr(rsrc.payerAddr), fast.AOValidAt(voucherValidAt))
	require.NoError(err)

	voucher, err := types.DecodeVoucher(voucherStr)
	require.NoError(err)

	assert.Equal(voucherAmount, &voucher.Amount)
}

func TestPaymentChannelRedeemSuccess(t *testing.T) {
	tf.IntegrationTest(t)

	require := require.New(t)
	assert := assert.New(t)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
	defer cancel()

	// Get basic testing environment
	ctx, env := fastesting.NewTestEnvironment(ctx, t, fast.EnvironmentOpts{})

	// Teardown after test ends
	defer func() {
		err := env.Teardown(ctx)
		require.NoError(err)
	}()

	// Start test
	rsrc := requireNewPaychResource(ctx, t, env)

	channelExpiry := types.NewBlockHeight(20)
	channelAmount := types.NewAttoFILFromFIL(1000)

	chanid, _ := rsrc.requirePaymentChannel(ctx, channelAmount, channelExpiry)

	voucherAmount := types.NewAttoFILFromFIL(10)
	voucherValidAt := types.NewBlockHeight(0)
	voucherStr, err := rsrc.payer.PaychVoucher(ctx, chanid, voucherAmount, fast.AOFromAddr(rsrc.payerAddr), fast.AOValidAt(voucherValidAt))
	require.NoError(err)

	mcid, err := rsrc.target.PaychRedeem(ctx, voucherStr, fast.AOFromAddr(rsrc.targetAddr), fast.AOPrice(big.NewFloat(1)), fast.AOLimit(300))
	require.NoError(err)

	series.CtxMiningOnce(ctx)

	resp, err := rsrc.target.MessageWait(ctx, mcid)
	require.NoError(err)
	assert.Equal(0, int(resp.Receipt.ExitCode))

	channels, err := rsrc.target.PaychLs(ctx, fast.AOFromAddr(rsrc.payerAddr))
	require.NoError(err)

	channel := channels[chanid.String()]
	assert.Equal(channelAmount, channel.Amount)
	assert.Equal(voucherAmount, channel.AmountRedeemed)
}

func TestPaymentChannelRedeemTooEarlyFails(t *testing.T) {
	tf.IntegrationTest(t)

	require := require.New(t)
	assert := assert.New(t)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
	defer cancel()

	// Get basic testing environment
	ctx, env := fastesting.NewTestEnvironment(ctx, t, fast.EnvironmentOpts{})

	// Teardown after test ends
	defer func() {
		err := env.Teardown(ctx)
		require.NoError(err)
	}()

	// Start test
	rsrc := requireNewPaychResource(ctx, t, env)

	channelExpiry := types.NewBlockHeight(20)
	channelAmount := types.NewAttoFILFromFIL(1000)

	chanid, _ := rsrc.requirePaymentChannel(ctx, channelAmount, channelExpiry)

	voucherAmount := types.NewAttoFILFromFIL(10)
	voucherValidAt := types.NewBlockHeight(10)
	voucherStr, err := rsrc.payer.PaychVoucher(ctx, chanid, voucherAmount, fast.AOFromAddr(rsrc.payerAddr), fast.AOValidAt(voucherValidAt))
	require.NoError(err)

	mcid, err := rsrc.target.PaychRedeem(ctx, voucherStr, fast.AOFromAddr(rsrc.targetAddr), fast.AOPrice(big.NewFloat(1)), fast.AOLimit(300))
	require.NoError(err)

	series.CtxMiningOnce(ctx)

	resp, err := rsrc.target.MessageWait(ctx, mcid)
	require.NoError(err)
	assert.Equal(paymentbroker.ErrTooEarly, int(resp.Receipt.ExitCode))

	channels, err := rsrc.target.PaychLs(ctx, fast.AOFromAddr(rsrc.payerAddr))
	require.NoError(err)

	channel := channels[chanid.String()]
	assert.Equal(channelAmount, channel.Amount)
	assert.Equal(types.ZeroAttoFIL, channel.AmountRedeemed)
}

func TestPaymentChannelReclaimSuccess(t *testing.T) {
	tf.IntegrationTest(t)

	require := require.New(t)
	assert := assert.New(t)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
	defer cancel()

	// Get basic testing environment
	ctx, env := fastesting.NewTestEnvironment(ctx, t, fast.EnvironmentOpts{})

	// Teardown after test ends
	defer func() {
		err := env.Teardown(ctx)
		require.NoError(err)
	}()

	// Start test
	rsrc := requireNewPaychResource(ctx, t, env)

	bh, err := series.GetHeadBlockHeight(ctx, env.GenesisMiner)
	require.NoError(err)

	// Expiry is current height, plus 3
	// - Setting up the payment channel
	// - Redeeming one voucher
	// - Expires on third block
	channelExpiry := types.NewBlockHeight(3).Add(bh)
	channelAmount := types.NewAttoFILFromFIL(1000)

	balanceBefore, err := rsrc.payer.WalletBalance(ctx, rsrc.payerAddr)
	require.NoError(err)

	chanid, gasReceipt := rsrc.requirePaymentChannel(ctx, channelAmount, channelExpiry)

	voucherAmount := types.NewAttoFILFromFIL(10)
	voucherValidAt := types.NewBlockHeight(0)
	voucherStr, err := rsrc.payer.PaychVoucher(ctx, chanid, voucherAmount, fast.AOFromAddr(rsrc.payerAddr), fast.AOValidAt(voucherValidAt))
	require.NoError(err)

	mcid, err := rsrc.target.PaychRedeem(ctx, voucherStr, fast.AOFromAddr(rsrc.targetAddr), fast.AOPrice(big.NewFloat(1)), fast.AOLimit(300))
	require.NoError(err)

	series.CtxMiningOnce(ctx)

	resp, err := rsrc.target.MessageWait(ctx, mcid)
	require.NoError(err)
	assert.Equal(0, int(resp.Receipt.ExitCode))

	channels, err := rsrc.target.PaychLs(ctx, fast.AOFromAddr(rsrc.payerAddr))
	require.NoError(err)

	channel := channels[chanid.String()]
	assert.Equal(channelAmount, channel.Amount)
	assert.Equal(voucherAmount, channel.AmountRedeemed)

	series.CtxMiningOnce(ctx)

	mcid, err = rsrc.payer.PaychReclaim(ctx, chanid, fast.AOFromAddr(rsrc.payerAddr), fast.AOPrice(big.NewFloat(1)), fast.AOLimit(300))
	require.NoError(err)

	series.CtxMiningOnce(ctx)

	resp, err = rsrc.payer.MessageWait(ctx, mcid)
	require.NoError(err)
	assert.Equal(0, int(resp.Receipt.ExitCode))
	gasReceipt = gasReceipt.Add(resp.Receipt.GasAttoFIL)

	channels, err = rsrc.payer.PaychLs(ctx, fast.AOFromAddr(rsrc.payerAddr))
	require.NoError(err)
	require.Len(channels, 0)

	balanceAfter, err := rsrc.payer.WalletBalance(ctx, rsrc.payerAddr)
	require.NoError(err)

	assert.Equal(balanceBefore.Sub(gasReceipt), balanceAfter.Add(voucherAmount))
}

func TestPaymentChannelCloseSuccess(t *testing.T) {
	tf.IntegrationTest(t)

	require := require.New(t)
	assert := assert.New(t)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
	defer cancel()

	// Get basic testing environment
	ctx, env := fastesting.NewTestEnvironment(ctx, t, fast.EnvironmentOpts{})

	// Teardown after test ends
	defer func() {
		err := env.Teardown(ctx)
		require.NoError(err)
	}()

	// Start test
	rsrc := requireNewPaychResource(ctx, t, env)

	channelExpiry := types.NewBlockHeight(5)
	channelAmount := types.NewAttoFILFromFIL(1000)

	payerBalanceBefore, err := rsrc.payer.WalletBalance(ctx, rsrc.payerAddr)
	require.NoError(err)

	targetBalanceBefore, err := rsrc.target.WalletBalance(ctx, rsrc.targetAddr)
	require.NoError(err)

	chanid, gasReceiptForPaychCreate := rsrc.requirePaymentChannel(ctx, channelAmount, channelExpiry)

	voucherAmount := types.NewAttoFILFromFIL(10)
	voucherValidAt := types.NewBlockHeight(0)
	voucherStr, err := rsrc.payer.PaychVoucher(ctx, chanid, voucherAmount, fast.AOFromAddr(rsrc.payerAddr), fast.AOValidAt(voucherValidAt))
	require.NoError(err)

	mcid, err := rsrc.target.PaychClose(ctx, voucherStr, fast.AOFromAddr(rsrc.targetAddr), fast.AOPrice(big.NewFloat(1)), fast.AOLimit(300))
	require.NoError(err)

	series.CtxMiningOnce(ctx)

	resp, err := rsrc.target.MessageWait(ctx, mcid)
	require.NoError(err)
	assert.Equal(0, int(resp.Receipt.ExitCode))

	channels, err := rsrc.target.PaychLs(ctx, fast.AOFromAddr(rsrc.payerAddr))
	require.NoError(err)
	require.Len(channels, 0)

	payerBalanceAfter, err := rsrc.payer.WalletBalance(ctx, rsrc.payerAddr)
	require.NoError(err)
	assert.Equal(payerBalanceBefore.Sub(voucherAmount).Sub(gasReceiptForPaychCreate), payerBalanceAfter)

	targetBalanceAfter, err := rsrc.target.WalletBalance(ctx, rsrc.targetAddr)
	require.NoError(err)
	assert.Equal(targetBalanceBefore.Add(voucherAmount).Sub(resp.Receipt.GasAttoFIL), targetBalanceAfter)
}

func TestPaymentChannelExtendSuccess(t *testing.T) {
	tf.IntegrationTest(t)

	require := require.New(t)
	assert := assert.New(t)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
	defer cancel()

	// Get basic testing environment
	ctx, env := fastesting.NewTestEnvironment(ctx, t, fast.EnvironmentOpts{})

	// Teardown after test ends
	defer func() {
		err := env.Teardown(ctx)
		require.NoError(err)
	}()

	// Start test
	rsrc := requireNewPaychResource(ctx, t, env)

	channelExpiry := types.NewBlockHeight(5)
	channelAmount := types.NewAttoFILFromFIL(1000)

	chanid, _ := rsrc.requirePaymentChannel(ctx, channelAmount, channelExpiry)

	channels, err := rsrc.payer.PaychLs(ctx)
	require.NoError(err)

	assert.Len(channels, 1)

	channel := channels[chanid.String()]
	assert.Equal(channelAmount, channel.Amount)
	assert.Equal(channelExpiry, channel.Eol)
	assert.Equal(rsrc.targetAddr, channel.Target)
	assert.Equal(types.ZeroAttoFIL, channel.AmountRedeemed)

	extendAmount := types.NewAttoFILFromFIL(100)
	extendExpiry := types.NewBlockHeight(100)

	mcid, err := rsrc.payer.PaychExtend(ctx, chanid, extendAmount, extendExpiry, fast.AOFromAddr(rsrc.payerAddr), fast.AOPrice(big.NewFloat(1)), fast.AOLimit(300))
	require.NoError(err)

	series.CtxMiningOnce(ctx)

	resp, err := rsrc.payer.MessageWait(ctx, mcid)
	require.NoError(err)
	assert.Equal(0, int(resp.Receipt.ExitCode))

	channels, err = rsrc.payer.PaychLs(ctx)
	require.NoError(err)

	assert.Len(channels, 1)

	channel = channels[chanid.String()]
	assert.Equal(channelAmount.Add(extendAmount), channel.Amount)
	assert.Equal(extendExpiry, channel.Eol)
	assert.Equal(rsrc.targetAddr, channel.Target)
	assert.Equal(types.ZeroAttoFIL, channel.AmountRedeemed)
}

type paychResources struct {
	t *testing.T

	target *fast.Filecoin
	payer  *fast.Filecoin

	targetAddr address.Address
	payerAddr  address.Address
}

func requireNewPaychResource(ctx context.Context, t *testing.T, env *fastesting.TestEnvironment) *paychResources {
	require := require.New(t)

	targetDaemon := env.RequireNewNodeWithFunds(10000)
	payerDaemon := env.RequireNewNodeWithFunds(10000)

	addrs, err := targetDaemon.AddressLs(ctx)
	require.NoError(err)
	targetAddr := addrs[0]

	addrs, err = payerDaemon.AddressLs(ctx)
	require.NoError(err)
	payerAddr := addrs[0]

	return &paychResources{
		t: t,

		target:     targetDaemon,
		targetAddr: targetAddr,

		payer:     payerDaemon,
		payerAddr: payerAddr,
	}
}

func (rsrc *paychResources) requirePaymentChannel(ctx context.Context, amt *types.AttoFIL, eol *types.BlockHeight) (*types.ChannelID, *types.AttoFIL) {
	require := require.New(rsrc.t)
	assert := assert.New(rsrc.t)

	mcid, err := rsrc.payer.PaychCreate(ctx, rsrc.targetAddr, amt, eol, fast.AOFromAddr(rsrc.payerAddr), fast.AOPrice(big.NewFloat(1)), fast.AOLimit(300))
	require.NoError(err)

	series.CtxMiningOnce(ctx)

	resp, err := rsrc.payer.MessageWait(ctx, mcid)
	require.NoError(err)
	assert.Equal(0, int(resp.Receipt.ExitCode))

	chanid := types.NewChannelIDFromBytes(resp.Receipt.Return[0])
	require.NotNil(chanid)

	return chanid, resp.Receipt.GasAttoFIL
}
