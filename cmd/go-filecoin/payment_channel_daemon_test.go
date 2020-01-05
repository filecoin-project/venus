package commands_test

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/fixtures"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	//	"github.com/filecoin-project/go-filecoin/tools/fast/series"
)

func TestPaymentChannelCreateSuccess(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()

	// Start test
	rsrc := requireNewPaychResource(ctx, t)

	channelExpiry := types.NewBlockHeight(20)
	channelAmount := types.NewAttoFILFromFIL(1000)

	rsrc.requirePaymentChannel(ctx, t, channelAmount, channelExpiry)
	rsrc.Stop(ctx)
}

func TestPaymentChannelLs(t *testing.T) {
	tf.IntegrationTest(t)

	t.Run("Works with default payer", func(t *testing.T) {
		ctx := context.Background()

		// Start test
		rsrc := requireNewPaychResource(ctx, t)
		defer rsrc.Stop(ctx)

		channelExpiry := types.NewBlockHeight(20)
		channelAmount := types.NewAttoFILFromFIL(1000)

		chanid, _ := rsrc.requirePaymentChannel(ctx, t, channelAmount, channelExpiry)

		channels, err := rsrc.payer.PorcelainAPI.PaymentChannelLs(ctx, rsrc.payerAddr, rsrc.payerAddr)
		require.NoError(t, err)

		assert.Len(t, channels, 1)

		channel := channels[chanid.String()]
		assert.Equal(t, channelAmount, channel.Amount)
		assert.Equal(t, channelExpiry, channel.AgreedEol)
		assert.Equal(t, channelExpiry, channel.Eol)
		assert.Equal(t, rsrc.targetAddr, channel.Target)
		assert.Equal(t, types.ZeroAttoFIL, channel.AmountRedeemed)
	})

	t.Run("Works with specified payer", func(t *testing.T) {
		ctx := context.Background()

		// Start test
		rsrc := requireNewPaychResource(ctx, t)
		defer rsrc.Stop(ctx)

		channelExpiry := types.NewBlockHeight(20)
		channelAmount := types.NewAttoFILFromFIL(1000)

		chanid, _ := rsrc.requirePaymentChannel(ctx, t, channelAmount, channelExpiry)

		channels, err := rsrc.payer.PorcelainAPI.PaymentChannelLs(ctx, rsrc.payerAddr, rsrc.payerAddr)
		require.NoError(t, err)

		assert.Len(t, channels, 1)

		channel := channels[chanid.String()]
		assert.Equal(t, channelAmount, channel.Amount)
		assert.Equal(t, channelExpiry, channel.AgreedEol)
		assert.Equal(t, channelExpiry, channel.Eol)
		assert.Equal(t, rsrc.targetAddr, channel.Target)
		assert.Equal(t, types.ZeroAttoFIL, channel.AmountRedeemed)
	})

	t.Run("No results when listing with different from address", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
		defer cancel()

		// Start test
		rsrc := requireNewPaychResource(ctx, t)
		defer rsrc.Stop(ctx)

		channelExpiry := types.NewBlockHeight(20)
		channelAmount := types.NewAttoFILFromFIL(1000)

		// requirePaymentChannel sets up a channel with the rsrc.payerAddr as the from address
		rsrc.requirePaymentChannel(ctx, t, channelAmount, channelExpiry)

		channels, err := rsrc.payer.PorcelainAPI.PaymentChannelLs(ctx, rsrc.payerAddr, rsrc.targetAddr)
		require.NoError(t, err)

		assert.Len(t, channels, 0)
	})
}

func TestPaymentChannelVoucherSuccess(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()

	// Start test
	rsrc := requireNewPaychResource(ctx, t)

	channelExpiry := types.NewBlockHeight(20)
	channelAmount := types.NewAttoFILFromFIL(1000)

	chanid, _ := rsrc.requirePaymentChannel(ctx, t, channelAmount, channelExpiry)

	voucherAmount := types.NewAttoFILFromFIL(10)
	voucherValidAt := types.NewBlockHeight(1)
	voucher, err := rsrc.payer.PorcelainAPI.PaymentChannelVoucher(ctx, rsrc.payerAddr, chanid, voucherAmount, voucherValidAt, nil)
	require.NoError(t, err)

	assert.Equal(t, voucherAmount, voucher.Amount)
	assert.True(t, voucherValidAt.Equal(&voucher.ValidAt))
	assert.Equal(t, rsrc.payerAddr, voucher.Payer)
	assert.Equal(t, rsrc.targetAddr, voucher.Target)
}

func TestPaymentChannelRedeemSuccess(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()

	// Start test
	rsrc := requireNewPaychResource(ctx, t)

	channelExpiry := types.NewBlockHeight(20)
	channelAmount := types.NewAttoFILFromFIL(1000)

	chanid, _ := rsrc.requirePaymentChannel(ctx, t, channelAmount, channelExpiry)

	voucherAmount := types.NewAttoFILFromFIL(10)
	voucherValidAt := types.NewBlockHeight(0)

	voucher, err := rsrc.payer.PorcelainAPI.PaymentChannelVoucher(ctx, rsrc.payerAddr, chanid, voucherAmount, voucherValidAt, nil)
	require.NoError(t, err)

	params := []interface{}{
		voucher.Payer,
		&voucher.Channel,
		voucher.Amount,
		&voucher.ValidAt,
		voucher.Condition,
		[]byte(voucher.Signature),
		[]interface{}{},
	}
	sender := newTestSender(t, types.NewAttoFIL(big.NewInt(1)), 300, rsrc.targetAddr, rsrc.target)
	mcid := sender.requireSend(ctx, address.PaymentBrokerAddress, types.NewAttoFILFromFIL(0), paymentbroker.Redeem, params...)

	_, err = rsrc.miner.PorcelainAPI.MessagePoolWait(ctx, 1)
	require.NoError(t, err)
	_, err = rsrc.miner.BlockMining.BlockMiningAPI.MiningOnce(ctx)
	require.NoError(t, err)

	rcpt, err := rsrc.target.PorcelainAPI.MessageWaitDone(ctx, mcid)
	require.NoError(t, err)
	assert.Equal(t, 0, int(rcpt.ExitCode))

	channels, err := rsrc.target.PorcelainAPI.PaymentChannelLs(ctx, rsrc.targetAddr, rsrc.payerAddr)
	require.NoError(t, err)

	channel := channels[chanid.String()]
	assert.Equal(t, channelAmount, channel.Amount)
	assert.Equal(t, voucherAmount, channel.AmountRedeemed)
}

func TestPaymentChannelRedeemTooEarlyFails(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()

	// Start test
	rsrc := requireNewPaychResource(ctx, t)

	channelExpiry := types.NewBlockHeight(20)
	channelAmount := types.NewAttoFILFromFIL(1000)

	chanid, _ := rsrc.requirePaymentChannel(ctx, t, channelAmount, channelExpiry)

	voucherAmount := types.NewAttoFILFromFIL(10)
	voucherValidAt := types.NewBlockHeight(10)
	voucher, err := rsrc.payer.PorcelainAPI.PaymentChannelVoucher(ctx, rsrc.payerAddr, chanid, voucherAmount, voucherValidAt, nil)
	require.NoError(t, err)

	params := []interface{}{
		voucher.Payer,
		&voucher.Channel,
		voucher.Amount,
		&voucher.ValidAt,
		voucher.Condition,
		[]byte(voucher.Signature),
		[]interface{}{},
	}
	sender := newTestSender(t, types.NewAttoFIL(big.NewInt(1)), 300, rsrc.targetAddr, rsrc.target)
	mcid := sender.requireSend(ctx, address.PaymentBrokerAddress, types.NewAttoFILFromFIL(0), paymentbroker.Redeem, params...)

	_, err = rsrc.miner.PorcelainAPI.MessagePoolWait(ctx, 1)
	require.NoError(t, err)
	_, err = rsrc.miner.BlockMining.BlockMiningAPI.MiningOnce(ctx)
	require.NoError(t, err)

	rcpt, err := rsrc.target.PorcelainAPI.MessageWaitDone(ctx, mcid)
	require.NoError(t, err)
	assert.Equal(t, paymentbroker.ErrTooEarly, int(rcpt.ExitCode))

	channels, err := rsrc.target.PorcelainAPI.PaymentChannelLs(ctx, rsrc.targetAddr, rsrc.payerAddr)
	require.NoError(t, err)

	channel := channels[chanid.String()]
	assert.Equal(t, channelAmount, channel.Amount)
	assert.Equal(t, types.ZeroAttoFIL, channel.AmountRedeemed)
}

func TestPaymentChannelReclaimSuccess(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()
	// Start test
	rsrc := requireNewPaychResource(ctx, t)

	head, err := rsrc.miner.PorcelainAPI.ChainHead()
	require.NoError(t, err)
	h, err := head.Height()
	require.NoError(t, err)
	bh := types.NewBlockHeight(h)

	// Expiry is current height, plus 3
	// - Setting up the payment channel
	// - Redeeming one voucher
	// - Expires on third block
	channelExpiry := types.NewBlockHeight(3).Add(bh)
	channelAmount := types.NewAttoFILFromFIL(1000)

	balanceBefore, err := rsrc.payer.PorcelainAPI.WalletBalance(ctx, rsrc.payerAddr)
	require.NoError(t, err)

	chanid, gasReceipt := rsrc.requirePaymentChannel(ctx, t, channelAmount, channelExpiry)

	voucherAmount := types.NewAttoFILFromFIL(10)
	voucherValidAt := types.NewBlockHeight(0)

	voucher, err := rsrc.payer.PorcelainAPI.PaymentChannelVoucher(ctx, rsrc.payerAddr, chanid, voucherAmount, voucherValidAt, nil)
	require.NoError(t, err)

	params := []interface{}{
		voucher.Payer,
		&voucher.Channel,
		voucher.Amount,
		&voucher.ValidAt,
		voucher.Condition,
		[]byte(voucher.Signature),
		[]interface{}{},
	}
	targetSender := newTestSender(t, types.NewAttoFIL(big.NewInt(1)), 300, rsrc.targetAddr, rsrc.target)
	mcid := targetSender.requireSend(ctx, address.PaymentBrokerAddress, types.NewAttoFILFromFIL(0), paymentbroker.Redeem, params...)

	_, err = rsrc.miner.PorcelainAPI.MessagePoolWait(ctx, 1)
	require.NoError(t, err)
	_, err = rsrc.miner.BlockMining.BlockMiningAPI.MiningOnce(ctx)
	require.NoError(t, err)

	rcpt, err := rsrc.target.PorcelainAPI.MessageWaitDone(ctx, mcid)
	require.NoError(t, err)
	assert.Equal(t, 0, int(rcpt.ExitCode))

	channels, err := rsrc.target.PorcelainAPI.PaymentChannelLs(ctx, rsrc.targetAddr, rsrc.payerAddr)
	require.NoError(t, err)

	channel := channels[chanid.String()]
	assert.Equal(t, channelAmount, channel.Amount)
	assert.Equal(t, voucherAmount, channel.AmountRedeemed)

	_, err = rsrc.miner.BlockMining.BlockMiningAPI.MiningOnce(ctx)
	require.NoError(t, err)

	payerSender := newTestSender(t, types.NewAttoFIL(big.NewInt(1)), 300, rsrc.payerAddr, rsrc.payer)
	mcid = payerSender.requireSend(ctx, address.PaymentBrokerAddress, types.NewAttoFILFromFIL(0), paymentbroker.Reclaim, chanid)

	_, err = rsrc.miner.PorcelainAPI.MessagePoolWait(ctx, 1)
	require.NoError(t, err)
	_, err = rsrc.miner.BlockMining.BlockMiningAPI.MiningOnce(ctx)
	require.NoError(t, err)

	rcpt, err = rsrc.payer.PorcelainAPI.MessageWaitDone(ctx, mcid)
	require.NoError(t, err)
	assert.Equal(t, 0, int(rcpt.ExitCode))
	gasReceipt = gasReceipt.Add(rcpt.GasAttoFIL)

	channels, err = rsrc.payer.PorcelainAPI.PaymentChannelLs(ctx, rsrc.payerAddr, rsrc.payerAddr)
	require.NoError(t, err)
	require.Len(t, channels, 0)

	balanceAfter, err := rsrc.payer.PorcelainAPI.WalletBalance(ctx, rsrc.payerAddr)
	require.NoError(t, err)

	assert.Equal(t, balanceBefore.Sub(gasReceipt), balanceAfter.Add(voucherAmount))
}

func TestPaymentChannelCloseSuccess(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()

	// Start test
	rsrc := requireNewPaychResource(ctx, t)

	channelExpiry := types.NewBlockHeight(5)
	channelAmount := types.NewAttoFILFromFIL(1000)

	payerBalanceBefore, err := rsrc.payer.PorcelainAPI.WalletBalance(ctx, rsrc.payerAddr)
	require.NoError(t, err)

	targetBalanceBefore, err := rsrc.target.PorcelainAPI.WalletBalance(ctx, rsrc.targetAddr)
	require.NoError(t, err)

	chanid, gasReceiptForPaychCreate := rsrc.requirePaymentChannel(ctx, t, channelAmount, channelExpiry)

	voucherAmount := types.NewAttoFILFromFIL(10)
	voucherValidAt := types.NewBlockHeight(0)

	voucher, err := rsrc.payer.PorcelainAPI.PaymentChannelVoucher(ctx, rsrc.payerAddr, chanid, voucherAmount, voucherValidAt, nil)
	require.NoError(t, err)

	params := []interface{}{
		voucher.Payer,
		&voucher.Channel,
		voucher.Amount,
		&voucher.ValidAt,
		voucher.Condition,
		[]byte(voucher.Signature),
		[]interface{}{},
	}
	sender := newTestSender(t, types.NewAttoFIL(big.NewInt(1)), 300, rsrc.targetAddr, rsrc.target)
	mcid := sender.requireSend(ctx, address.PaymentBrokerAddress, types.NewAttoFILFromFIL(0), paymentbroker.Close, params...)

	require.NoError(t, err)

	_, err = rsrc.miner.PorcelainAPI.MessagePoolWait(ctx, 1)
	require.NoError(t, err)
	_, err = rsrc.miner.BlockMining.BlockMiningAPI.MiningOnce(ctx)
	require.NoError(t, err)

	rcpt, err := rsrc.target.PorcelainAPI.MessageWaitDone(ctx, mcid)
	require.NoError(t, err)
	assert.Equal(t, 0, int(rcpt.ExitCode))

	channels, err := rsrc.target.PorcelainAPI.PaymentChannelLs(ctx, rsrc.targetAddr, rsrc.payerAddr)
	require.NoError(t, err)
	require.Len(t, channels, 0)

	// payer must wait for close message to see correct balance
	_, err = rsrc.payer.PorcelainAPI.MessageWaitDone(ctx, mcid)
	require.NoError(t, err)

	payerBalanceAfter, err := rsrc.payer.PorcelainAPI.WalletBalance(ctx, rsrc.payerAddr)
	require.NoError(t, err)
	assert.Equal(t, payerBalanceBefore.Sub(voucherAmount).Sub(gasReceiptForPaychCreate), payerBalanceAfter)

	targetBalanceAfter, err := rsrc.target.PorcelainAPI.WalletBalance(ctx, rsrc.targetAddr)
	require.NoError(t, err)
	assert.Equal(t, targetBalanceBefore.Add(voucherAmount).Sub(rcpt.GasAttoFIL), targetBalanceAfter)
}

func TestPaymentChannelExtendSuccess(t *testing.T) {
	tf.IntegrationTest(t)

	ctx := context.Background()

	// Start test
	rsrc := requireNewPaychResource(ctx, t)

	channelExpiry := types.NewBlockHeight(5)
	channelAmount := types.NewAttoFILFromFIL(1000)

	chanid, _ := rsrc.requirePaymentChannel(ctx, t, channelAmount, channelExpiry)

	channels, err := rsrc.payer.PorcelainAPI.PaymentChannelLs(ctx, rsrc.payerAddr, rsrc.payerAddr)
	require.NoError(t, err)

	assert.Len(t, channels, 1)

	channel := channels[chanid.String()]
	assert.Equal(t, channelAmount, channel.Amount)
	assert.Equal(t, channelExpiry, channel.AgreedEol)
	assert.Equal(t, channelExpiry, channel.Eol)
	assert.Equal(t, rsrc.targetAddr, channel.Target)
	assert.Equal(t, types.ZeroAttoFIL, channel.AmountRedeemed)

	extendAmount := types.NewAttoFILFromFIL(100)
	extendExpiry := types.NewBlockHeight(100)

	sender := newTestSender(t, types.NewAttoFIL(big.NewInt(1)), 300, rsrc.payerAddr, rsrc.payer)
	mcid := sender.requireSend(
		ctx,
		address.PaymentBrokerAddress,
		extendAmount,
		paymentbroker.Extend,
		chanid, extendExpiry,
	)

	_, err = rsrc.miner.PorcelainAPI.MessagePoolWait(ctx, 1)
	require.NoError(t, err)
	_, err = rsrc.miner.BlockMining.BlockMiningAPI.MiningOnce(ctx)
	require.NoError(t, err)

	rcpt, err := rsrc.target.PorcelainAPI.MessageWaitDone(ctx, mcid)
	require.NoError(t, err)
	assert.Equal(t, 0, int(rcpt.ExitCode))

	channels, err = rsrc.target.PorcelainAPI.PaymentChannelLs(ctx, rsrc.targetAddr, rsrc.payerAddr)
	require.NoError(t, err)

	assert.Len(t, channels, 1)

	channel = channels[chanid.String()]
	assert.Equal(t, channelAmount.Add(extendAmount), channel.Amount)
	assert.Equal(t, extendExpiry, channel.AgreedEol)
	assert.Equal(t, extendExpiry, channel.Eol)
	assert.Equal(t, rsrc.targetAddr, channel.Target)
	assert.Equal(t, types.ZeroAttoFIL, channel.AmountRedeemed)
}

func TestPaymentChannelCancelSuccess(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()
	// Start test
	rsrc := requireNewPaychResource(ctx, t)
	defer rsrc.Stop(ctx)
	channelExpiry := types.NewBlockHeight(20000)
	chanid, _ := rsrc.requirePaymentChannel(ctx, t, types.NewAttoFILFromFIL(1000), channelExpiry)
	channels, err := rsrc.payer.PorcelainAPI.PaymentChannelLs(ctx, rsrc.payerAddr, address.Undef)
	require.NoError(t, err)

	assert.Len(t, channels, 1)

	channel := channels[chanid.String()]
	assert.Equal(t, channelExpiry, channel.AgreedEol)
	assert.Equal(t, channelExpiry, channel.Eol)

	require.NoError(t, err)
	sender := newTestSender(t, types.NewAttoFIL(big.NewInt(1)), 300, rsrc.payerAddr, rsrc.payer)
	mcid := sender.requireSend(
		ctx,
		address.PaymentBrokerAddress,
		types.NewAttoFIL(big.NewInt(1)),
		paymentbroker.Cancel,
		chanid,
	)

	_, err = rsrc.miner.PorcelainAPI.MessagePoolWait(ctx, 1)
	require.NoError(t, err)
	_, err = rsrc.miner.BlockMining.BlockMiningAPI.MiningOnce(ctx)
	require.NoError(t, err)

	rcpt, err := rsrc.payer.PorcelainAPI.MessageWaitDone(ctx, mcid)
	require.NoError(t, err)
	assert.Equal(t, 0, int(rcpt.ExitCode))

	channels, err = rsrc.payer.PorcelainAPI.PaymentChannelLs(ctx, rsrc.payerAddr, rsrc.payerAddr)
	require.NoError(t, err)

	assert.Len(t, channels, 1)

	channel = channels[chanid.String()]
	assert.Equal(t, channelExpiry, channel.AgreedEol)
	assert.Equal(t, types.NewBlockHeight(10002), channel.Eol)
}

type paychResources struct {
	t *testing.T

	target *node.Node
	payer  *node.Node
	miner  *node.Node

	targetAddr address.Address
	payerAddr  address.Address
}

func requireNewPaychResource(ctx context.Context, t *testing.T) *paychResources {
	cs := node.FixtureChainSeed(t)

	builder := test.NewNodeBuilder(t)
	buildWithMiner(t, builder)

	builder1 := test.NewNodeBuilder(t)
	defaultAddr1, err := address.NewFromString(fixtures.TestAddresses[1])
	require.NoError(t, err)
	builder1.WithGenesisInit(cs.GenesisInitFunc)
	builder1.WithConfig(node.DefaultAddressConfigOpt(defaultAddr1))
	builder1.WithInitOpt(cs.KeyInitOpt(1))

	builder2 := test.NewNodeBuilder(t)
	defaultAddr2, err := address.NewFromString(fixtures.TestAddresses[2])
	require.NoError(t, err)
	builder2.WithGenesisInit(cs.GenesisInitFunc)
	builder2.WithConfig(node.DefaultAddressConfigOpt(defaultAddr2))
	builder2.WithInitOpt(cs.KeyInitOpt(2))

	minerNode := builder.BuildAndStart(ctx)
	targetNode := builder1.BuildAndStart(ctx)
	payerNode := builder2.BuildAndStart(ctx)

	node.ConnectNodes(t, minerNode, targetNode)
	node.ConnectNodes(t, targetNode, payerNode)
	node.ConnectNodes(t, payerNode, minerNode)

	return &paychResources{
		t: t,

		target: targetNode,
		payer:  payerNode,
		miner:  minerNode,

		targetAddr: defaultAddr1,
		payerAddr:  defaultAddr2,
	}
}

func (rsrc *paychResources) requirePaymentChannel(ctx context.Context, t *testing.T, amt types.AttoFIL, eol *types.BlockHeight) (*types.ChannelID, types.AttoFIL) {
	sender := newTestSender(t, types.NewAttoFIL(big.NewInt(1)), 300, rsrc.payerAddr, rsrc.payer)
	mcid := sender.requireSend(ctx, address.PaymentBrokerAddress, amt, paymentbroker.CreateChannel, rsrc.targetAddr, eol)

	_, err := rsrc.miner.PorcelainAPI.MessagePoolWait(ctx, 1)
	require.NoError(t, err)
	_, err = rsrc.miner.BlockMining.BlockMiningAPI.MiningOnce(ctx)
	require.NoError(t, err)

	rcpt, err := rsrc.payer.PorcelainAPI.MessageWaitDone(ctx, mcid)
	require.NoError(t, err)
	assert.Equal(t, 0, int(rcpt.ExitCode))

	chanid := types.NewChannelIDFromBytes(rcpt.Return[0])
	require.NotNil(t, chanid)

	return chanid, rcpt.GasAttoFIL
}

func (rsrc *paychResources) Stop(ctx context.Context) {
	rsrc.miner.Stop(ctx)
	rsrc.target.Stop(ctx)
	rsrc.payer.Stop(ctx)
}

// messageSender is a test helper that makes message sending less verbose
type messageSender struct {
	t        *testing.T
	gasPrice types.AttoFIL
	gasLimit types.Uint64
	from     address.Address

	node *node.Node
}

func (ms *messageSender) requireSend(ctx context.Context, to address.Address, val types.AttoFIL, method types.MethodID, params ...interface{}) cid.Cid {
	mcid, _, err := ms.node.PorcelainAPI.MessageSend(
		ctx,
		ms.from,
		to,
		val,
		ms.gasPrice,
		ms.gasLimit,
		method,
		params...,
	)
	require.NoError(ms.t, err)
	return mcid
}

func newTestSender(t *testing.T, gp types.AttoFIL, gl types.Uint64, f address.Address, n *node.Node) *messageSender {
	return &messageSender{
		t:        t,
		gasPrice: gp,
		gasLimit: gl,
		from:     f,
		node:     n,
	}
}
