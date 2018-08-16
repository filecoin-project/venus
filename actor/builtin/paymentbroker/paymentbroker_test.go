package paymentbroker_test

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"

	cbor "gx/ipfs/QmSyK1ZiAP98YvnxsTfQpb669V2xeTHRbG4Y6fgKS3vVSd/go-ipld-cbor"
	"gx/ipfs/QmbwwhSsEcSPP4XfGumu6GMcuCLnCLVQAnp3mDxKuYNXJo/go-hamt-ipld"
	"gx/ipfs/QmcD7SqfyQyA91TZUQ7VPRYbGarxmY7EsQewVYMuN5LNSv/go-ipfs-blockstore"
	"gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	. "github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaymentBrokerGenesis(t *testing.T) {
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, st, vms := requireGenesis(ctx, t, types.NewAddressForTestGetter()())

	paymentBroker := state.MustGetActor(st, address.PaymentBrokerAddress)

	assert.Equal(types.NewAttoFILFromFIL(0), paymentBroker.Balance)

	var pbStorage State
	builtin.RequireReadState(t, vms, address.PaymentBrokerAddress, paymentBroker, &pbStorage)
	assert.Equal(0, len(pbStorage.Channels))
}

func TestPaymentBrokerCreateChannel(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payer := address.TestAddress
	target := types.NewAddressForTestGetter()()
	_, st, vms := requireGenesis(ctx, t, target)

	pdata := core.MustConvertParams(target, big.NewInt(10))
	msg := types.NewMessage(payer, address.PaymentBrokerAddress, 0, types.NewAttoFILFromFIL(1000), "createChannel", pdata)

	result, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	require.NoError(err)

	channelID := big.NewInt(0)
	channelID.SetBytes(result.Receipt.Return[0])

	paymentBroker := state.MustGetActor(st, address.PaymentBrokerAddress)

	assert.Equal(types.NewAttoFILFromFIL(1000), paymentBroker.Balance)

	var pbStorage State
	builtin.RequireReadState(t, vms, address.PaymentBrokerAddress, paymentBroker, &pbStorage)

	require.Equal(1, len(pbStorage.Channels))
	require.Equal(1, len(pbStorage.Channels[payer.String()]))
	byPayer := pbStorage.Channels[payer.String()]

	channel := byPayer[channelID.String()]
	require.NotNil(channel)

	assert.Equal(types.NewAttoFILFromFIL(1000), channel.Amount)
	assert.Equal(types.NewAttoFILFromFIL(0), channel.AmountRedeemed)
	assert.Equal(target, channel.Target)
	assert.Equal(types.NewBlockHeight(10), channel.Eol)
}

func TestPaymentBrokerCreateChannelFromNonAccountActorIsAnError(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payee := types.NewAddressForTestGetter()()
	_, st, vms := requireGenesis(ctx, t, payee)

	// Create a non-account actor
	payerActor := types.NewActor(types.NewCidForTestGetter()(), types.NewAttoFILFromFIL(2000))
	payer := types.NewAddressForTestGetter()()
	state.MustSetActor(st, payer, payerActor)

	pdata := core.MustConvertParams(payee, big.NewInt(10))
	msg := types.NewMessage(payer, address.PaymentBrokerAddress, 0, types.NewAttoFILFromFIL(1000), "createChannel", pdata)
	_, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))

	// expect error
	require.Error(err)
}

func TestPaymentBrokerUpdate(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payer := address.TestAddress
	target := types.NewAddressForTestGetter()()
	_, st, vms := requireGenesis(ctx, t, target)

	channelID := establishChannel(ctx, st, vms, payer, target, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))

	// channel creator's address is our signature for now.
	pdata := core.MustConvertParams(payer, channelID, types.NewAttoFILFromFIL(100), payer)
	msg := types.NewMessage(target, address.PaymentBrokerAddress, 0, types.NewAttoFILFromFIL(0), "update", pdata)
	result, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	require.NoError(err)
	require.Equal(uint8(0), result.Receipt.ExitCode)

	paymentBroker := state.MustGetActor(st, address.PaymentBrokerAddress)

	assert.Equal(types.NewAttoFILFromFIL(900), paymentBroker.Balance)

	payee := state.MustGetActor(st, target)

	assert.Equal(types.NewAttoFILFromFIL(100), payee.Balance)

	channel := retrieveChannel(t, vms, paymentBroker, payer, channelID)

	assert.Equal(types.NewAttoFILFromFIL(1000), channel.Amount)
	assert.Equal(types.NewAttoFILFromFIL(100), channel.AmountRedeemed)
	assert.Equal(target, channel.Target)
}

func TestPaymentBrokerUpdateErrorsWithIncorrectChannel(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payer := address.TestAddress
	addressGetter := types.NewAddressForTestGetter()
	target := addressGetter()
	_, st, vms := requireGenesis(ctx, t, target)

	channelID := establishChannel(ctx, st, vms, payer, target, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))

	// update message from payer instead of target results in error
	pdata := core.MustConvertParams(payer, channelID, types.NewAttoFILFromFIL(100), payer)
	msg := types.NewMessage(payer, address.PaymentBrokerAddress, 1, types.NewAttoFILFromFIL(0), "update", pdata)

	result, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	require.NoError(err)

	require.NotEqual(uint8(0), result.Receipt.ExitCode)

	// invalid channel id results in revert error
	pdata = core.MustConvertParams(payer, types.NewChannelID(39932), types.NewAttoFILFromFIL(100), payer)
	msg = types.NewMessage(target, address.PaymentBrokerAddress, 0, types.NewAttoFILFromFIL(0), "update", pdata)

	result, err = core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	require.NoError(err)

	require.NotEqual(uint8(0), result.Receipt.ExitCode)
	require.Contains(result.ExecutionError.Error(), "payment channel")
}

func TestPaymentBrokerUpdateErrorsWhenNotFromTarget(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payer := address.TestAddress
	addressGetter := types.NewAddressForTestGetter()
	payeeAddress := addressGetter()

	_, st, vms := requireGenesis(ctx, t, payeeAddress)

	wrongTargetAddress := addressGetter()
	wrongTargetActor := core.RequireNewAccountActor(require, types.NewAttoFILFromFIL(0))
	st.SetActor(ctx, wrongTargetAddress, wrongTargetActor)

	channelID := establishChannel(ctx, st, vms, payer, payeeAddress, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))

	// message originating from actor other than channel results in revert error
	pdata := core.MustConvertParams(payer, channelID, types.NewAttoFILFromFIL(100), payer)
	msg := types.NewMessage(wrongTargetAddress, address.PaymentBrokerAddress, 0, types.NewAttoFILFromFIL(0), "update", pdata)

	result, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	require.NoError(err)

	require.NotEqual(uint8(0), result.Receipt.ExitCode)
	require.Contains(result.ExecutionError.Error(), "wrong target account")
}

func TestPaymentBrokerUpdateErrorsWhenRedeemingMoreThanChannelContains(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payer := address.TestAddress
	payeeAddress := types.NewAddressForTestGetter()()
	_, st, vms := requireGenesis(ctx, t, payeeAddress)

	channelID := establishChannel(ctx, st, vms, payer, payeeAddress, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))

	// redeeming more than channel contains is an error
	pdata := core.MustConvertParams(payer, channelID, types.NewAttoFILFromFIL(1100), payer)
	msg := types.NewMessage(payeeAddress, address.PaymentBrokerAddress, 0, types.NewAttoFILFromFIL(0), "update", pdata)

	result, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	require.NoError(err)

	require.NotEqual(uint8(0), result.Receipt.ExitCode)
	require.Contains(result.ExecutionError.Error(), "exceeds amount")
}

func TestPaymentBrokerUpdateErrorsWhenRedeemingFundsAlreadyRedeemed(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payer := address.TestAddress
	payeeAddress := types.NewAddressForTestGetter()()
	_, st, vms := requireGenesis(ctx, t, payeeAddress)

	channelID := establishChannel(ctx, st, vms, payer, payeeAddress, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))

	// redeem some
	pdata := core.MustConvertParams(payer, channelID, types.NewAttoFILFromFIL(500), payer)
	msg := types.NewMessage(payeeAddress, address.PaymentBrokerAddress, 0, types.NewAttoFILFromFIL(0), "update", pdata)

	result, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	require.NoError(err)

	require.Equal(uint8(0), result.Receipt.ExitCode)

	// redeeming funds already redeemed is an error
	pdata = core.MustConvertParams(payer, channelID, types.NewAttoFILFromFIL(400), payer)
	msg = types.NewMessage(payeeAddress, address.PaymentBrokerAddress, 1, types.NewAttoFILFromFIL(0), "update", pdata)

	result, err = core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	require.NoError(err)

	require.NotEqual(uint8(0), result.Receipt.ExitCode)
	require.Contains(result.ExecutionError.Error(), "update amount")
}

func TestPaymentBrokerUpdateErrorsWhenAtEol(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payer := address.TestAddress
	payeeAddress := types.NewAddressForTestGetter()()
	_, st, vms := requireGenesis(ctx, t, payeeAddress)

	channelID := establishChannel(ctx, st, vms, payer, payeeAddress, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))

	pdata := core.MustConvertParams(payer, channelID, types.NewAttoFILFromFIL(100), payer)
	msg := types.NewMessage(payeeAddress, address.PaymentBrokerAddress, 0, types.NewAttoFILFromFIL(0), "update", pdata)

	// set block height to Eol
	result, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(10))
	require.NoError(err)

	// expect an error
	assert.NotEqual(uint8(0), result.Receipt.ExitCode)
	assert.True(strings.Contains(strings.ToLower(result.ExecutionError.Error()), "block height"), "Error should relate to block height")
}

func TestPaymentBrokerClose(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payerAddress := address.TestAddress
	targetAddress := types.NewAddressForTestGetter()()
	_, st, vms := requireGenesis(ctx, t, targetAddress)

	channelID := establishChannel(ctx, st, vms, payerAddress, targetAddress, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))

	payerActor := state.MustGetActor(st, payerAddress)

	payerBalancePriorToClose := payerActor.Balance

	// channel creator's address is our signature for now.
	pdata := core.MustConvertParams(payerAddress, channelID, types.NewAttoFILFromFIL(100), payerAddress)
	msg := types.NewMessage(targetAddress, address.PaymentBrokerAddress, 0, types.NewAttoFILFromFIL(0), "close", pdata)
	_, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	require.NoError(err)

	paymentBroker := state.MustGetActor(st, address.PaymentBrokerAddress)

	// all funds have been redeemed or returned
	assert.Equal(types.NewAttoFILFromFIL(0), paymentBroker.Balance)

	targetActor := state.MustGetActor(st, targetAddress)

	// targetActor has been paid
	assert.Equal(types.NewAttoFILFromFIL(100), targetActor.Balance)

	// remaining balance is returned to payer
	payerActor = state.MustGetActor(st, payerAddress)
	assert.Equal(payerBalancePriorToClose.Add(types.NewAttoFILFromFIL(900)), payerActor.Balance)
}

func TestPaymentBrokerReclaim(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payerAddress := address.TestAddress
	payeeAddress := types.NewAddressForTestGetter()()
	_, st, vms := requireGenesis(ctx, t, payeeAddress)

	channelID := establishChannel(ctx, st, vms, payerAddress, payeeAddress, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))

	payer := state.MustGetActor(st, payerAddress)
	payerBalancePriorToClose := payer.Balance

	pdata := core.MustConvertParams(channelID)
	msg := types.NewMessage(payerAddress, address.PaymentBrokerAddress, 1, types.NewAttoFILFromFIL(0), "reclaim", pdata)
	// block height is after Eol
	_, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(11))
	require.NoError(err)

	paymentBroker := state.MustGetActor(st, address.PaymentBrokerAddress)

	// all funds have been redeemed or returned
	assert.Equal(types.NewAttoFILFromFIL(0), paymentBroker.Balance)

	// entire balance is returned to payer
	payer = state.MustGetActor(st, payerAddress)
	assert.Equal(payerBalancePriorToClose.Add(types.NewAttoFILFromFIL(1000)), payer.Balance)
}

func TestPaymentBrokerReclaimFailsBeforeChannelEol(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payerAddress := address.TestAddress
	targetAddress := types.NewAddressForTestGetter()()
	_, st, vms := requireGenesis(ctx, t, targetAddress)

	channelID := establishChannel(ctx, st, vms, payerAddress, targetAddress, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))

	pdata := core.MustConvertParams(channelID)
	msg := types.NewMessage(payerAddress, address.PaymentBrokerAddress, 1, types.NewAttoFILFromFIL(0), "reclaim", pdata)
	// block height is before Eol
	result, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	require.NoError(err)

	// fails
	assert.NotEqual(uint8(0), result.Receipt.ExitCode)
	assert.Contains(result.ExecutionError.Error(), "reclaim")
	assert.Contains(result.ExecutionError.Error(), "eol")
}

func TestPaymentBrokerExtend(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payer := address.TestAddress
	target := types.NewAddressForTestGetter()()
	_, st, vms := requireGenesis(ctx, t, target)

	channelID := establishChannel(ctx, st, vms, payer, target, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))

	// extend channel
	pdata := core.MustConvertParams(channelID, types.NewBlockHeight(20))
	msg := types.NewMessage(payer, address.PaymentBrokerAddress, 1, types.NewAttoFILFromFIL(1000), "extend", pdata)

	result, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(9))
	require.NoError(err)
	assert.Equal(uint8(0), result.Receipt.ExitCode)

	// try to request too high an amount after the eol for the original channel
	pdata = core.MustConvertParams(payer, channelID, types.NewAttoFILFromFIL(1100), payer)
	msg = types.NewMessage(target, address.PaymentBrokerAddress, 0, types.NewAttoFILFromFIL(0), "update", pdata)
	result, err = core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(12))

	// expect success
	require.NoError(err)
	assert.Equal(uint8(0), result.Receipt.ExitCode)

	// check memory
	paymentBroker := state.MustGetActor(st, address.PaymentBrokerAddress)
	assert.Equal(types.NewAttoFILFromFIL(900), paymentBroker.Balance) // 1000 + 1000 - 1100

	var pbStorage State
	builtin.RequireReadState(t, vms, address.PaymentBrokerAddress, paymentBroker, &pbStorage)

	byPayer := pbStorage.Channels[payer.String()]
	channel := byPayer[channelID.String()]
	assert.Equal(types.NewAttoFILFromFIL(2000), channel.Amount)
	assert.Equal(types.NewAttoFILFromFIL(1100), channel.AmountRedeemed)
	assert.Equal(types.NewBlockHeight(20), channel.Eol)
}

func TestPaymentBrokerExtendFailsWithNonExistantChannel(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payer := address.TestAddress
	payeeAddress := types.NewAddressForTestGetter()()
	_, st, vms := requireGenesis(ctx, t, payeeAddress)

	_ = establishChannel(ctx, st, vms, payer, payeeAddress, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))

	// extend channel
	pdata := core.MustConvertParams(types.NewChannelID(383), types.NewBlockHeight(20))
	msg := types.NewMessage(payer, address.PaymentBrokerAddress, 1, types.NewAttoFILFromFIL(1000), "extend", pdata)

	result, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(9))
	require.NoError(err)
	assert.NotEqual(uint8(0), result.Receipt.ExitCode)
}

func TestPaymentBrokerExtendRefusesToShortenTheEol(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payer := address.TestAddress
	payeeAddress := types.NewAddressForTestGetter()()
	_, st, vms := requireGenesis(ctx, t, payeeAddress)

	channelID := establishChannel(ctx, st, vms, payer, payeeAddress, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))

	// extend channel setting block height to 5 (<10)
	pdata := core.MustConvertParams(channelID, types.NewBlockHeight(5))
	msg := types.NewMessage(payer, address.PaymentBrokerAddress, 1, types.NewAttoFILFromFIL(1000), "extend", pdata)

	result, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(9))
	require.NoError(err)

	assert.NotEqual(uint8(0), result.Receipt.ExitCode)
	assert.Contains(result.ExecutionError.Error(), "payment channel eol may not be decreased")
}

func TestPaymentBrokerLs(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	t.Run("Successfully returns channels", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		payer := address.TestAddress
		target1 := types.NewAddressForTestGetter()()
		target2 := types.NewAddressForTestGetter()()
		_, st, vms := requireGenesis(ctx, t, target1)
		targetActor2 := core.RequireNewAccountActor(require, types.NewAttoFILFromFIL(0))
		st.SetActor(ctx, target2, targetActor2)

		channelId1 := establishChannel(ctx, st, vms, payer, target1, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))
		channelId2 := establishChannel(ctx, st, vms, payer, target2, 1, types.NewAttoFILFromFIL(2000), types.NewBlockHeight(20))

		// retrieve channels
		args, err := abi.ToEncodedValues(payer)
		require.NoError(err)

		returnValue, exitCode, err := core.CallQueryMethod(ctx, st, vms, address.PaymentBrokerAddress, "ls", args, payer, types.NewBlockHeight(9))
		require.NoError(err)
		assert.Equal(uint8(0), exitCode)

		channels := make(map[string]*PaymentChannel)
		err = cbor.DecodeInto(returnValue[0], &channels)
		require.NoError(err)

		assert.Equal(2, len(channels))

		pc1, found := channels[channelId1.String()]
		require.True(found)
		assert.Equal(target1, pc1.Target)
		assert.Equal(types.NewAttoFILFromFIL(1000), pc1.Amount)
		assert.Equal(types.NewAttoFILFromFIL(0), pc1.AmountRedeemed)
		assert.Equal(types.NewBlockHeight(10), pc1.Eol)

		pc2, found := channels[channelId2.String()]
		require.True(found)
		assert.Equal(target2, pc2.Target)
		assert.Equal(types.NewAttoFILFromFIL(2000), pc2.Amount)
		assert.Equal(types.NewAttoFILFromFIL(0), pc2.AmountRedeemed)
		assert.Equal(types.NewBlockHeight(20), pc2.Eol)
	})

	t.Run("Returns empty map when payer has no channels", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		payer := address.TestAddress
		target1 := types.NewAddressForTestGetter()()
		_, st, vms := requireGenesis(ctx, t, target1)

		// retrieve channels
		args, err := abi.ToEncodedValues(payer)
		require.NoError(err)

		returnValue, exitCode, err := core.CallQueryMethod(ctx, st, vms, address.PaymentBrokerAddress, "ls", args, payer, types.NewBlockHeight(9))
		require.NoError(err)
		assert.Equal(uint8(0), exitCode)

		channels := make(map[string]*PaymentChannel)
		err = cbor.DecodeInto(returnValue[0], &channels)
		require.NoError(err)

		assert.Equal(0, len(channels))
	})
}

func TestNewPaymentBrokerVoucher(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	t.Run("Returns valid voucher", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		payer := address.TestAddress
		target := types.NewAddressForTestGetter()()
		_, st, vms := requireGenesis(ctx, t, target)

		channelID := establishChannel(ctx, st, vms, payer, target, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))

		// create voucher
		voucherAmount := types.NewAttoFILFromFIL(100)
		pdata := core.MustConvertParams(channelID, voucherAmount)

		returnValue, exitCode, err := core.CallQueryMethod(ctx, st, vms, address.PaymentBrokerAddress, "voucher", pdata, payer, types.NewBlockHeight(9))
		require.NoError(err)
		assert.Equal(uint8(0), exitCode)

		voucher := PaymentVoucher{}
		err = cbor.DecodeInto(returnValue[0], &voucher)
		require.NoError(err)

		assert.Equal(*channelID, voucher.Channel)
		assert.Equal(payer, voucher.Payer)
		assert.Equal(target, voucher.Target)
		assert.Equal(*voucherAmount, voucher.Amount)
	})

	t.Run("Errors when channel does not exist", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		payer := address.TestAddress
		target := types.NewAddressForTestGetter()()
		_, st, vms := requireGenesis(ctx, t, target)

		_ = establishChannel(ctx, st, vms, payer, target, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))
		notChannelID := types.NewChannelID(999)

		// create voucher
		voucherAmount := types.NewAttoFILFromFIL(100)
		args := core.MustConvertParams(notChannelID, voucherAmount)

		_, exitCode, err := core.CallQueryMethod(ctx, st, vms, address.PaymentBrokerAddress, "voucher", args, payer, types.NewBlockHeight(9))
		assert.NotEqual(uint8(0), exitCode)
		assert.Contains(fmt.Sprintf("%v", err), "unknown")
	})

	t.Run("Errors when voucher exceed channel amount", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		payer := address.TestAddress
		target := types.NewAddressForTestGetter()()
		_, st, vms := requireGenesis(ctx, t, target)

		channelID := establishChannel(ctx, st, vms, payer, target, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))

		// create voucher
		voucherAmount := types.NewAttoFILFromFIL(2000)
		args := core.MustConvertParams(channelID, voucherAmount)

		_, exitCode, err := core.CallQueryMethod(ctx, st, vms, address.PaymentBrokerAddress, "voucher", args, payer, types.NewBlockHeight(9))
		assert.NotEqual(uint8(0), exitCode)
		assert.Contains(fmt.Sprintf("%v", err), "exceeds amount")
	})
}

func establishChannel(ctx context.Context, st state.Tree, vms vm.StorageMap, from types.Address, target types.Address, nonce uint64, amt *types.AttoFIL, eol *types.BlockHeight) *types.ChannelID {
	pdata := core.MustConvertParams(target, eol)
	msg := types.NewMessage(from, address.PaymentBrokerAddress, nonce, amt, "createChannel", pdata)
	result, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	if err != nil {
		panic(err)
	}

	channelID := types.NewChannelIDFromBytes(result.Receipt.Return[0])
	return channelID
}

func retrieveChannel(t *testing.T, vms vm.StorageMap, paymentBroker *types.Actor, payer types.Address, channelID *types.ChannelID) *PaymentChannel {
	require := require.New(t)

	var pbStorage State
	builtin.RequireReadState(t, vms, address.PaymentBrokerAddress, paymentBroker, &pbStorage)

	require.Equal(1, len(pbStorage.Channels))
	require.Equal(1, len(pbStorage.Channels[payer.String()]))
	byPayer := pbStorage.Channels[payer.String()]

	channel := byPayer[channelID.String()]
	require.NotNil(channel)
	return channel
}

func requireGenesis(ctx context.Context, t *testing.T, targetAddress types.Address) (*hamt.CborIpldStore, state.Tree, vm.StorageMap) {
	require := require.New(t)

	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	vms := vm.NewStorageMap(bs)

	cst := hamt.NewCborStore()
	blk, err := core.InitGenesis(cst, bs)
	require.NoError(err)

	st, err := state.LoadStateTree(ctx, cst, blk.StateRoot, builtin.Actors)
	require.NoError(err)

	targetActor := core.RequireNewAccountActor(require, types.NewAttoFILFromFIL(0))

	st.SetActor(ctx, targetAddress, targetActor)

	return cst, st, vms
}
