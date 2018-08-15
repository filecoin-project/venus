package paymentbroker_test

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"

	cbor "gx/ipfs/QmPbqRavwDZLfmpeW6eoyAoQ5rT2LoCW98JhvRc22CqkZS/go-ipld-cbor"
	"gx/ipfs/QmSkuaNgyGmV8c1L3cZNWcUxRJV6J3nsD96JVQPcWcwtyW/go-hamt-ipld"
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

var ki = types.MustGenerateKeyInfo(10, types.GenerateKeyInfoSeed())
var mockSigner = types.NewMockSigner(ki)

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
	require.NoError(result.ExecutionError)
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

	payer := mockSigner.Addresses[0]
	target := types.NewAddressForTestGetter()()
	_, st, vms := requireGenesis(ctx, t, target)

	payerActor := core.RequireNewAccountActor(require, types.NewAttoFILFromFIL(50000))
	state.MustSetActor(st, payer, payerActor)

	channelID := establishChannel(ctx, st, vms, payer, target, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))

	amt := types.NewAttoFILFromFIL(100)
	data := append(channelID.Bytes(), amt.Bytes()...)
	signature, err := mockSigner.SignBytes(data, payer)
	require.NoError(err)

	pdata := core.MustConvertParams(payer, channelID, amt, ([]byte)(signature))
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

	payer := mockSigner.Addresses[0]
	addressGetter := types.NewAddressForTestGetter()
	target := addressGetter()
	_, st, vms := requireGenesis(ctx, t, target)
	payerActor := core.RequireNewAccountActor(require, types.NewAttoFILFromFIL(50000))
	state.MustSetActor(st, payer, payerActor)

	channelID := establishChannel(ctx, st, vms, payer, target, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))
	amt := types.NewAttoFILFromFIL(100)

	data := append(channelID.Bytes(), amt.Bytes()...)
	signature, err := mockSigner.SignBytes(data, payer)
	require.NoError(err)

	// update message from payer instead of target results in error
	pdata := core.MustConvertParams(payer, channelID, amt, ([]byte)(signature))
	msg := types.NewMessage(payer, address.PaymentBrokerAddress, 1, types.NewAttoFILFromFIL(0), "update", pdata)

	result, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	require.NoError(err)

	require.NotEqual(uint8(0), result.Receipt.ExitCode)

	// invalid channel id results in revert error
	pdata = core.MustConvertParams(payer, types.NewChannelID(39932), amt, ([]byte)(signature))
	msg = types.NewMessage(target, address.PaymentBrokerAddress, 0, types.NewAttoFILFromFIL(0), "update", pdata)

	result, err = core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	require.NoError(err)

	require.NotEqual(uint8(0), result.Receipt.ExitCode)
	require.Contains(result.ExecutionError.Error(), "signature failed")
}

func TestPaymentBrokerUpdateErrorsWhenNotFromTarget(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payer := mockSigner.Addresses[0]
	addressGetter := types.NewAddressForTestGetter()
	payeeAddress := addressGetter()

	_, st, vms := requireGenesis(ctx, t, payeeAddress)
	payerActor := core.RequireNewAccountActor(require, types.NewAttoFILFromFIL(50000))
	state.MustSetActor(st, payer, payerActor)

	wrongTargetAddress := addressGetter()
	wrongTargetActor := core.RequireNewAccountActor(require, types.NewAttoFILFromFIL(0))
	st.SetActor(ctx, wrongTargetAddress, wrongTargetActor)

	channelID := establishChannel(ctx, st, vms, payer, payeeAddress, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))

	amt := types.NewAttoFILFromFIL(100)
	data := append(channelID.Bytes(), amt.Bytes()...)
	signature, err := mockSigner.SignBytes(data, payer)
	require.NoError(err)

	// message originating from actor other than channel results in revert error
	pdata := core.MustConvertParams(payer, channelID, amt, ([]byte)(signature))
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

	payer := mockSigner.Addresses[0]
	payeeAddress := types.NewAddressForTestGetter()()
	_, st, vms := requireGenesis(ctx, t, payeeAddress)
	payerActor := core.RequireNewAccountActor(require, types.NewAttoFILFromFIL(50000))
	state.MustSetActor(st, payer, payerActor)

	channelID := establishChannel(ctx, st, vms, payer, payeeAddress, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))

	amt := types.NewAttoFILFromFIL(1100)
	data := append(channelID.Bytes(), amt.Bytes()...)
	signature, err := mockSigner.SignBytes(data, payer)
	require.NoError(err)

	// redeeming more than channel contains is an error
	pdata := core.MustConvertParams(payer, channelID, amt, ([]byte)(signature))
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

	payer := mockSigner.Addresses[0]
	payeeAddress := types.NewAddressForTestGetter()()
	_, st, vms := requireGenesis(ctx, t, payeeAddress)
	payerActor := core.RequireNewAccountActor(require, types.NewAttoFILFromFIL(50000))
	state.MustSetActor(st, payer, payerActor)

	channelID := establishChannel(ctx, st, vms, payer, payeeAddress, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))

	amt1 := types.NewAttoFILFromFIL(500)
	data1 := append(channelID.Bytes(), amt1.Bytes()...)
	signature1, err := mockSigner.SignBytes(data1, payer)
	require.NoError(err)

	// redeem some
	pdata := core.MustConvertParams(payer, channelID, amt1, ([]byte)(signature1))
	msg := types.NewMessage(payeeAddress, address.PaymentBrokerAddress, 0, types.NewAttoFILFromFIL(0), "update", pdata)

	result, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	require.NoError(result.ExecutionError)
	require.NoError(err)

	require.Equal(uint8(0), result.Receipt.ExitCode)

	amt2 := types.NewAttoFILFromFIL(400)
	data2 := append(channelID.Bytes(), amt2.Bytes()...)
	signature2, err := mockSigner.SignBytes(data2, payer)
	require.NoError(err)

	// redeeming funds already redeemed is an error
	pdata = core.MustConvertParams(payer, channelID, amt2, ([]byte)(signature2))
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

	payer := mockSigner.Addresses[0]
	payeeAddress := types.NewAddressForTestGetter()()
	_, st, vms := requireGenesis(ctx, t, payeeAddress)
	payerActor := core.RequireNewAccountActor(require, types.NewAttoFILFromFIL(50000))
	state.MustSetActor(st, payer, payerActor)

	channelID := establishChannel(ctx, st, vms, payer, payeeAddress, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))

	amt := types.NewAttoFILFromFIL(500)
	data := append(channelID.Bytes(), amt.Bytes()...)
	signature, err := mockSigner.SignBytes(data, payer)
	require.NoError(err)

	pdata := core.MustConvertParams(payer, channelID, amt, ([]byte)(signature))
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

	payerAddress := mockSigner.Addresses[0]
	targetAddress := types.NewAddressForTestGetter()()
	_, st, vms := requireGenesis(ctx, t, targetAddress)

	payerActor := core.RequireNewAccountActor(require, types.NewAttoFILFromFIL(50000))
	state.MustSetActor(st, payerAddress, payerActor)

	channelID := establishChannel(ctx, st, vms, payerAddress, targetAddress, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))

	amt := types.NewAttoFILFromFIL(100)
	payerActor = state.MustGetActor(st, payerAddress)
	payerBalancePriorToClose := payerActor.Balance

	data := append(channelID.Bytes(), amt.Bytes()...)
	signature, err := mockSigner.SignBytes(data, payerAddress)
	require.NoError(err)

	pdata := core.MustConvertParams(payerAddress, channelID, amt, ([]byte)(signature))
	msg := types.NewMessage(targetAddress, address.PaymentBrokerAddress, 0, types.NewAttoFILFromFIL(0), "close", pdata)
	res, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	require.NoError(res.ExecutionError)
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

func TestPaymentBrokerCloseInvalidSig(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payerAddress := mockSigner.Addresses[0]
	targetAddress := types.NewAddressForTestGetter()()
	_, st, vms := requireGenesis(ctx, t, targetAddress)

	payerActor := core.RequireNewAccountActor(require, types.NewAttoFILFromFIL(50000))
	state.MustSetActor(st, payerAddress, payerActor)

	channelID := establishChannel(ctx, st, vms, payerAddress, targetAddress, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))

	amt := types.NewAttoFILFromFIL(100)

	data := append(channelID.Bytes(), amt.Bytes()...)
	signature, err := mockSigner.SignBytes(data, payerAddress)
	require.NoError(err)
	// make the signature invalid
	signature[0] = 0
	signature[1] = 1

	pdata := core.MustConvertParams(payerAddress, channelID, amt, ([]byte)(signature))
	msg := types.NewMessage(targetAddress, address.PaymentBrokerAddress, 0, types.NewAttoFILFromFIL(0), "close", pdata)
	res, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	require.EqualError(res.ExecutionError, Errors[ErrInvalidSignature].Error())
	require.NoError(err)
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
	res, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(11))
	require.NoError(res.ExecutionError)
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

	payer := mockSigner.Addresses[0]
	target := types.NewAddressForTestGetter()()
	_, st, vms := requireGenesis(ctx, t, target)
	payerActor := core.RequireNewAccountActor(require, types.NewAttoFILFromFIL(50000))
	state.MustSetActor(st, payer, payerActor)

	channelID := establishChannel(ctx, st, vms, payer, target, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))

	// extend channel
	pdata := core.MustConvertParams(channelID, types.NewBlockHeight(20))
	msg := types.NewMessage(payer, address.PaymentBrokerAddress, 1, types.NewAttoFILFromFIL(1000), "extend", pdata)

	result, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(9))
	require.NoError(result.ExecutionError)
	require.NoError(err)
	assert.Equal(uint8(0), result.Receipt.ExitCode)

	amt := types.NewAttoFILFromFIL(1100)
	data := append(channelID.Bytes(), amt.Bytes()...)
	signature, err := mockSigner.SignBytes(data, payer)
	require.NoError(err)

	// try to request too high an amount after the eol for the original channel
	pdata = core.MustConvertParams(payer, channelID, amt, ([]byte)(signature))
	msg = types.NewMessage(target, address.PaymentBrokerAddress, 0, types.NewAttoFILFromFIL(0), "update", pdata)
	result, err = core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(12))
	require.NoError(result.ExecutionError)

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
	require.EqualError(result.ExecutionError, "payment channel is unknown")
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

		msg := types.NewMessage(payer, address.PaymentBrokerAddress, 1, nil, "voucher", pdata)
		res, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(9))
		assert.NoError(err)
		assert.NoError(res.ExecutionError)
		assert.Equal(uint8(0), res.Receipt.ExitCode)

		voucher := PaymentVoucher{}
		err = cbor.DecodeInto(res.Receipt.Return[0], &voucher)
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

		msg := types.NewMessage(payer, address.PaymentBrokerAddress, 1, nil, "voucher", args)
		res, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(9))
		assert.NoError(err)
		assert.NotEqual(uint8(0), res.Receipt.ExitCode)
		assert.Contains(fmt.Sprintf("%s", res.ExecutionError), "exceeds amount")
	})
}

func establishChannel(ctx context.Context, st state.Tree, vms vm.StorageMap, from types.Address, target types.Address, nonce uint64, amt *types.AttoFIL, eol *types.BlockHeight) *types.ChannelID {
	pdata := core.MustConvertParams(target, eol)
	msg := types.NewMessage(from, address.PaymentBrokerAddress, nonce, amt, "createChannel", pdata)
	result, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	if err != nil {
		panic(err)
	}

	if result.ExecutionError != nil {
		panic(result.ExecutionError)
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

func requireGenesis(ctx context.Context, t *testing.T, targetAddresses ...types.Address) (*hamt.CborIpldStore, state.Tree, vm.StorageMap) {
	require := require.New(t)

	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	vms := vm.NewStorageMap(bs)

	cst := hamt.NewCborStore()
	blk, err := core.InitGenesis(cst, bs)
	require.NoError(err)

	st, err := state.LoadStateTree(ctx, cst, blk.StateRoot, builtin.Actors)
	require.NoError(err)

	for _, addr := range targetAddresses {
		targetActor := core.RequireNewAccountActor(require, types.NewAttoFILFromFIL(0))
		st.SetActor(ctx, addr, targetActor)
	}

	return cst, st, vms
}
