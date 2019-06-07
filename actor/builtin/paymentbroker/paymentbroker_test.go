package paymentbroker_test

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	. "github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
	"github.com/filecoin-project/go-filecoin/vm/errors"
)

var mockSigner, _ = types.NewMockSignersAndKeyInfo(10)

var pbTestActorCid = types.NewCidForTestGetter()()

func TestPaymentBrokerGenesis(t *testing.T) {
	tf.UnitTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, st, _ := requireGenesis(ctx, t, address.NewForTestGetter()())

	paymentBroker := state.MustGetActor(st, address.PaymentBrokerAddress)

	assert.Equal(t, types.NewAttoFILFromFIL(0), paymentBroker.Balance)
}

func TestPaymentBrokerCreateChannel(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	payer := address.TestAddress
	target := address.NewForTestGetter()()
	_, st, vms := requireGenesis(ctx, t, target)

	pdata := core.MustConvertParams(target, big.NewInt(10))
	msg := types.NewMessage(payer, address.PaymentBrokerAddress, 0, types.NewAttoFILFromFIL(1000), "createChannel", pdata)

	result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))
	require.NoError(t, err)
	require.NoError(t, result.ExecutionError)

	st.Flush(ctx)

	channelID := types.NewChannelIDFromBytes(result.Receipt.Return[0])

	paymentBroker := state.MustGetActor(st, address.PaymentBrokerAddress)

	assert.Equal(t, types.NewAttoFILFromFIL(1000), paymentBroker.Balance)

	channel := requireGetPaymentChannel(t, ctx, st, vms, payer, channelID)

	assert.Equal(t, types.NewAttoFILFromFIL(1000), channel.Amount)
	assert.Equal(t, types.NewAttoFILFromFIL(0), channel.AmountRedeemed)
	assert.Equal(t, target, channel.Target)
	assert.Equal(t, types.NewBlockHeight(10), channel.AgreedEol)
	assert.Equal(t, types.NewBlockHeight(10), channel.Eol)
}

func TestPaymentBrokerUpdate(t *testing.T) {
	tf.UnitTest(t)

	sys := setup(t)

	result, err := sys.ApplyRedeemMessage(sys.target, 100, 0)
	require.NoError(t, err)
	require.Equal(t, uint8(0), result.Receipt.ExitCode)

	paymentBroker := state.MustGetActor(sys.st, address.PaymentBrokerAddress)

	assert.Equal(t, types.NewAttoFILFromFIL(900), paymentBroker.Balance)

	payee := state.MustGetActor(sys.st, sys.target)

	assert.Equal(t, types.NewAttoFILFromFIL(100), payee.Balance)

	channel := sys.retrieveChannel(paymentBroker)

	assert.Equal(t, types.NewAttoFILFromFIL(1000), channel.Amount)
	assert.Equal(t, types.NewAttoFILFromFIL(100), channel.AmountRedeemed)
	assert.Equal(t, sys.target, channel.Target)
}

func TestPaymentBrokerRedeemWithCondition(t *testing.T) {
	tf.UnitTest(t)

	addrGetter := address.NewForTestGetter()
	toAddress := addrGetter()
	method := "paramsNotZero"
	addrParam := addrGetter()
	sectorIdParam := uint64(6)
	payerParams := []interface{}{addrParam, sectorIdParam}
	blockHeightParam := types.NewBlockHeight(43)
	redeemerParams := []interface{}{blockHeightParam}

	// All the following tests attempt to call PBTestActor.ParamsNotZero with a condition.
	// PBTestActor.ParamsNotZero takes 3 parameter: an Address, a uint64 sector id, and a BlockHeight
	// If any of these are zero values the method throws an error indicating the condition is false.
	// The Address and the sector id will be included within the condition predicate, and the block
	// height will be added as a redeemer supplied parameter to redeem.

	t.Run("Redeem should succeed if condition is met", func(t *testing.T) {
		sys := setup(t)
		require.NoError(t, sys.st.SetActor(context.TODO(), toAddress, actor.NewActor(pbTestActorCid, types.ZeroAttoFIL)))

		condition := &types.Predicate{To: toAddress, Method: method, Params: payerParams}
		appResult, err := sys.applySignatureMessage(sys.target, 100, types.NewBlockHeight(0), 0, "redeem", 0, condition, redeemerParams...)

		require.NoError(t, err)
		require.NoError(t, appResult.ExecutionError)
	})

	t.Run("Redeem should fail if condition is _NOT_ met", func(t *testing.T) {
		sys := setup(t)
		require.NoError(t, sys.st.SetActor(context.TODO(), toAddress, actor.NewActor(pbTestActorCid, types.ZeroAttoFIL)))

		badAddressParam := address.Undef
		badParams := []interface{}{badAddressParam, sectorIdParam}

		condition := &types.Predicate{To: toAddress, Method: method, Params: badParams}
		appResult, err := sys.applySignatureMessage(sys.target, 100, types.NewBlockHeight(0), 0, "redeem", 0, condition, redeemerParams...)

		require.NoError(t, err)
		require.Error(t, appResult.ExecutionError)
		assert.Contains(t, appResult.ExecutionError.Error(), "failed to validate voucher condition: got undefined address")
		assert.EqualValues(t, errors.CodeError(appResult.ExecutionError), ErrConditionInvalid)
	})

	t.Run("Redeem should fail if condition goes to non-existent actor", func(t *testing.T) {
		sys := setup(t)
		require.NoError(t, sys.st.SetActor(context.TODO(), toAddress, actor.NewActor(pbTestActorCid, types.ZeroAttoFIL)))

		badToAddress := addrGetter()

		condition := &types.Predicate{To: badToAddress, Method: method, Params: payerParams}
		appResult, err := sys.applySignatureMessage(sys.target, 100, types.NewBlockHeight(0), 0, "redeem", 0, condition, redeemerParams...)

		require.NoError(t, err)
		require.Error(t, appResult.ExecutionError)
		assert.Contains(t, appResult.ExecutionError.Error(), "failed to validate voucher condition: actor code not found")
		assert.EqualValues(t, errors.CodeError(appResult.ExecutionError), ErrConditionInvalid)
	})

	t.Run("Redeem should fail if condition goes to non-existent method", func(t *testing.T) {
		sys := setup(t)
		require.NoError(t, sys.st.SetActor(context.TODO(), toAddress, actor.NewActor(pbTestActorCid, types.ZeroAttoFIL)))

		badMethod := "nonexistentMethod"

		condition := &types.Predicate{To: toAddress, Method: badMethod, Params: payerParams}
		appResult, err := sys.applySignatureMessage(sys.target, 100, types.NewBlockHeight(0), 0, "redeem", 0, condition, redeemerParams...)

		require.NoError(t, err)
		require.Error(t, appResult.ExecutionError)
		assert.Contains(t, appResult.ExecutionError.Error(), "failed to validate voucher condition: actor does not export method")
		assert.EqualValues(t, errors.CodeError(appResult.ExecutionError), ErrConditionInvalid)
	})

	t.Run("Redeem should fail if condition has the wrong number of condition parameters", func(t *testing.T) {
		sys := setup(t)
		require.NoError(t, sys.st.SetActor(context.TODO(), toAddress, actor.NewActor(pbTestActorCid, types.ZeroAttoFIL)))

		badParams := []interface{}{}

		condition := &types.Predicate{To: toAddress, Method: method, Params: badParams}
		appResult, err := sys.applySignatureMessage(sys.target, 100, types.NewBlockHeight(0), 0, "redeem", 0, condition, redeemerParams...)

		require.NoError(t, err)
		require.Error(t, appResult.ExecutionError)
		assert.Contains(t, appResult.ExecutionError.Error(), "failed to validate voucher condition: invalid params")
		assert.EqualValues(t, errors.CodeError(appResult.ExecutionError), ErrConditionInvalid)
	})

	t.Run("Redeem should fail if condition has the wrong number of supplied parameters", func(t *testing.T) {
		sys := setup(t)
		require.NoError(t, sys.st.SetActor(context.TODO(), toAddress, actor.NewActor(pbTestActorCid, types.ZeroAttoFIL)))

		badRedeemerParams := []interface{}{}

		condition := &types.Predicate{To: toAddress, Method: method, Params: payerParams}
		appResult, err := sys.applySignatureMessage(sys.target, 100, types.NewBlockHeight(0), 0, "redeem", 0, condition, badRedeemerParams...)

		require.NoError(t, err)
		require.Error(t, appResult.ExecutionError)
		assert.Contains(t, appResult.ExecutionError.Error(), "failed to validate voucher condition: invalid params")
		assert.EqualValues(t, errors.CodeError(appResult.ExecutionError), ErrConditionInvalid)
	})
}

func TestPaymentBrokerRedeemSetsConditionAndRedeemed(t *testing.T) {
	tf.UnitTest(t)

	addrGetter := address.NewForTestGetter()
	toAddress := addrGetter()
	method := "paramsNotZero"
	addrParam := addrGetter()
	sectorIdParam := uint64(6)
	payerParams := []interface{}{addrParam, sectorIdParam}
	blockHeightParam := types.NewBlockHeight(43)
	redeemerParams := []interface{}{blockHeightParam}

	t.Run("Redeem should set the redeemed flag to true on success", func(t *testing.T) {
		sys := setup(t)
		require.NoError(t, sys.st.SetActor(context.TODO(), toAddress, actor.NewActor(pbTestActorCid, types.ZeroAttoFIL)))

		// Expect that the redeemed flag is false on init
		paymentBroker := state.MustGetActor(sys.st, address.PaymentBrokerAddress)
		channel := sys.retrieveChannel(paymentBroker)
		assert.Equal(t, false, channel.Redeemed)

		// Successfully redeem the payment channel
		condition := &types.Predicate{To: toAddress, Method: method, Params: payerParams}
		appResult, err := sys.applySignatureMessage(sys.target, 100, types.NewBlockHeight(0), 0, "redeem", 0, condition, redeemerParams...)
		require.NoError(t, err)
		require.NoError(t, appResult.ExecutionError)

		// Expect that the redeemed flag is now true
		paymentBroker = state.MustGetActor(sys.st, address.PaymentBrokerAddress)
		channel = sys.retrieveChannel(paymentBroker)
		assert.Equal(t, true, channel.Redeemed)
	})

	t.Run("Redeem should set cached condition on success", func(t *testing.T) {
		sys := setup(t)
		require.NoError(t, sys.st.SetActor(context.TODO(), toAddress, actor.NewActor(pbTestActorCid, types.ZeroAttoFIL)))

		// Expect that the condition is nil on init
		paymentBroker := state.MustGetActor(sys.st, address.PaymentBrokerAddress)
		channel := sys.retrieveChannel(paymentBroker)
		assert.Nil(t, channel.Condition)

		// Successfully redeem the payment channel
		condition := &types.Predicate{To: toAddress, Method: method, Params: payerParams}
		appResult, err := sys.applySignatureMessage(sys.target, 100, types.NewBlockHeight(0), 0, "redeem", 0, condition, redeemerParams...)
		require.NoError(t, err)
		require.NoError(t, appResult.ExecutionError)

		// Expect that the condition is now set and correct
		paymentBroker = state.MustGetActor(sys.st, address.PaymentBrokerAddress)
		channel = sys.retrieveChannel(paymentBroker)
		assert.NotNil(t, channel.Condition)
		assert.Equal(t, toAddress, channel.Condition.To)
		assert.Equal(t, method, channel.Condition.Method)
		assert.Contains(t, channel.Condition.Params, addrParam.Bytes())
		assert.Contains(t, channel.Condition.Params, sectorIdParam)
		assert.Contains(t, channel.Condition.Params, blockHeightParam.Bytes())
	})

	t.Run("Redeem should set cached condition back to nil when no condition is provided", func(t *testing.T) {
		sys := setup(t)
		require.NoError(t, sys.st.SetActor(context.TODO(), toAddress, actor.NewActor(pbTestActorCid, types.ZeroAttoFIL)))

		// Successfully redeem the payment channel with condition
		condition := &types.Predicate{To: toAddress, Method: method, Params: payerParams}
		appResult, err := sys.applySignatureMessage(sys.target, 100, types.NewBlockHeight(0), 0, "redeem", 0, condition, redeemerParams...)
		require.NoError(t, err)
		require.NoError(t, appResult.ExecutionError)

		// Expect that the condition is set and correct
		paymentBroker := state.MustGetActor(sys.st, address.PaymentBrokerAddress)
		channel := sys.retrieveChannel(paymentBroker)
		assert.NotNil(t, channel.Condition)
		assert.Equal(t, toAddress, channel.Condition.To)
		assert.Equal(t, method, channel.Condition.Method)

		// Successfully redeem the payment channel again without condition
		appResult, err = sys.applySignatureMessage(sys.target, 200, types.NewBlockHeight(0), 0, "redeem", 0, nil, redeemerParams...)
		require.NoError(t, err)
		require.NoError(t, appResult.ExecutionError)

		// Expect that the condition is now nil
		paymentBroker = state.MustGetActor(sys.st, address.PaymentBrokerAddress)
		channel = sys.retrieveChannel(paymentBroker)
		assert.Nil(t, channel.Condition)
	})

	t.Run("Redeem should update the cached condition with new params when initial redeem condition is nil", func(t *testing.T) {
		sys := setup(t)
		require.NoError(t, sys.st.SetActor(context.TODO(), toAddress, actor.NewActor(pbTestActorCid, types.ZeroAttoFIL)))
		condition := &types.Predicate{To: toAddress, Method: method, Params: payerParams}

		// Successfully redeem the payment channel with no condition
		appResult, err := sys.applySignatureMessage(sys.target, 100, types.NewBlockHeight(0), 0, "redeem", 0, nil, redeemerParams...)
		require.NoError(t, err)
		require.NoError(t, appResult.ExecutionError)

		// Expect that the condition is nil
		paymentBroker := state.MustGetActor(sys.st, address.PaymentBrokerAddress)
		channel := sys.retrieveChannel(paymentBroker)
		assert.Nil(t, channel.Condition)

		// Successfully redeem the payment channel again with a condition
		appResult, err = sys.applySignatureMessage(sys.target, 200, types.NewBlockHeight(0), 0, "redeem", 0, condition, redeemerParams...)
		require.NoError(t, err)
		require.NoError(t, appResult.ExecutionError)

		// Expect that the condition is updated with the new redeemer params
		paymentBroker = state.MustGetActor(sys.st, address.PaymentBrokerAddress)
		channel = sys.retrieveChannel(paymentBroker)
		assert.NotNil(t, channel.Condition)
		assert.Equal(t, toAddress, channel.Condition.To)
		assert.Equal(t, method, channel.Condition.Method)
		assert.Contains(t, channel.Condition.Params, addrParam.Bytes())
		assert.Contains(t, channel.Condition.Params, sectorIdParam)
		assert.Contains(t, channel.Condition.Params, blockHeightParam.Bytes())
	})

	t.Run("Redeem should update the cached condition with new params when provided", func(t *testing.T) {
		sys := setup(t)
		require.NoError(t, sys.st.SetActor(context.TODO(), toAddress, actor.NewActor(pbTestActorCid, types.ZeroAttoFIL)))
		condition := &types.Predicate{To: toAddress, Method: method, Params: payerParams}

		// Successfully redeem the payment channel with condition
		appResult, err := sys.applySignatureMessage(sys.target, 100, types.NewBlockHeight(0), 0, "redeem", 0, condition, redeemerParams...)
		require.NoError(t, err)
		require.NoError(t, appResult.ExecutionError)

		// Expect that the condition is set and correct
		paymentBroker := state.MustGetActor(sys.st, address.PaymentBrokerAddress)
		channel := sys.retrieveChannel(paymentBroker)
		assert.NotNil(t, channel.Condition)
		assert.Equal(t, toAddress, channel.Condition.To)
		assert.Equal(t, method, channel.Condition.Method)

		// Successfully redeem the payment channel again with new redeemer params
		newBlockHeightParam := types.NewBlockHeight(52)
		newRedeemerParams := []interface{}{newBlockHeightParam}
		appResult, err = sys.applySignatureMessage(sys.target, 200, types.NewBlockHeight(0), 0, "redeem", 0, condition, newRedeemerParams...)
		require.NoError(t, err)
		require.NoError(t, appResult.ExecutionError)

		// Expect that the condition is updated with the new redeemer params
		paymentBroker = state.MustGetActor(sys.st, address.PaymentBrokerAddress)
		channel = sys.retrieveChannel(paymentBroker)
		assert.NotNil(t, channel.Condition)
		assert.Equal(t, toAddress, channel.Condition.To)
		assert.Equal(t, method, channel.Condition.Method)
		assert.Contains(t, channel.Condition.Params, addrParam.Bytes())
		assert.Contains(t, channel.Condition.Params, sectorIdParam)
		assert.Contains(t, channel.Condition.Params, newBlockHeightParam.Bytes())
	})

	t.Run("Redeem uses cached condition in subsequent calls", func(t *testing.T) {
		sys := setup(t)
		require.NoError(t, sys.st.SetActor(context.TODO(), toAddress, actor.NewActor(pbTestActorCid, types.ZeroAttoFIL)))

		// Redeem without params expects an invalid condition error
		condition := &types.Predicate{To: toAddress, Method: method}
		appResult, err := sys.applySignatureMessage(sys.target, 200, types.NewBlockHeight(0), 0, "redeem", 0, condition)
		require.NoError(t, err)
		require.Error(t, appResult.ExecutionError)
		require.EqualValues(t, errors.CodeError(appResult.ExecutionError), ErrConditionInvalid)

		// Successfully redeem the payment channel with params
		condition = &types.Predicate{To: toAddress, Method: method, Params: payerParams}
		appResult, err = sys.applySignatureMessage(sys.target, 100, types.NewBlockHeight(0), 0, "redeem", 0, condition, redeemerParams...)
		require.NoError(t, err)
		require.NoError(t, appResult.ExecutionError)

		// Redeem again without params and expect no error
		condition = &types.Predicate{To: toAddress, Method: method}
		appResult, err = sys.applySignatureMessage(sys.target, 200, types.NewBlockHeight(0), 0, "redeem", 0, condition)
		assert.NoError(t, err)
		assert.NoError(t, appResult.ExecutionError)
	})
}

func TestPaymentBrokerRedeemReversesCancellations(t *testing.T) {
	tf.UnitTest(t)

	sys := setup(t)

	// Cancel the payment channel
	pdata := core.MustConvertParams(sys.channelID)
	msg := types.NewMessage(sys.payer, address.PaymentBrokerAddress, 1, types.NewAttoFILFromFIL(1000), "cancel", pdata)
	result, err := sys.ApplyMessage(msg, 100)
	require.NoError(t, result.ExecutionError)
	require.NoError(t, err)
	require.Equal(t, uint8(0), result.Receipt.ExitCode)

	// Expect that the EOL of the payment channel now reflects the cancellation
	paymentBroker := state.MustGetActor(sys.st, address.PaymentBrokerAddress)
	channel := sys.retrieveChannel(paymentBroker)
	assert.Equal(t, types.NewBlockHeight(20000), channel.AgreedEol)
	assert.Equal(t, types.NewBlockHeight(10100), channel.Eol)

	// Redeem the payment channel
	result, err = sys.ApplyRedeemMessageWithBlockHeight(sys.target, 500, 0, 10000)
	require.NoError(t, err)

	// Expect that the EOL has been reset to its originally agreed upon value
	// meaning that the cancellation has been reversed
	paymentBroker = state.MustGetActor(sys.st, address.PaymentBrokerAddress)
	channel = sys.retrieveChannel(paymentBroker)
	assert.Equal(t, types.NewBlockHeight(20000), channel.AgreedEol)
	assert.Equal(t, types.NewBlockHeight(20000), channel.Eol)
}

func TestPaymentBrokerUpdateErrorsWithIncorrectChannel(t *testing.T) {
	tf.UnitTest(t)

	sys := setup(t)

	// update message from payer instead of target results in error
	result, err := sys.ApplyRedeemMessage(sys.payer, 100, 1)
	require.NoError(t, err)

	require.NotEqual(t, uint8(0), result.Receipt.ExitCode)

	// invalid channel id results in revert error
	sys.channelID = types.NewChannelID(39932)
	result, err = sys.ApplyRedeemMessage(sys.target, 100, 0)
	require.NoError(t, err)

	require.NotEqual(t, uint8(0), result.Receipt.ExitCode)
	require.Contains(t, result.ExecutionError.Error(), "unknown")
}

func TestPaymentBrokerUpdateErrorsWhenNotFromTarget(t *testing.T) {
	tf.UnitTest(t)

	sys := setup(t)

	wrongTargetAddress := sys.addressGetter()
	wrongTargetActor := th.RequireNewAccountActor(t, types.NewAttoFILFromFIL(0))
	sys.st.SetActor(sys.ctx, wrongTargetAddress, wrongTargetActor)

	result, err := sys.ApplyRedeemMessage(wrongTargetAddress, 100, 0)
	require.NoError(t, err)

	require.NotEqual(t, uint8(0), result.Receipt.ExitCode)
	require.Contains(t, result.ExecutionError.Error(), "wrong target account")
}

func TestPaymentBrokerUpdateErrorsWhenRedeemingMoreThanChannelContains(t *testing.T) {
	tf.UnitTest(t)

	sys := setup(t)

	result, err := sys.ApplyRedeemMessage(sys.target, 1100, 0)
	require.NoError(t, err)

	require.NotEqual(t, uint8(0), result.Receipt.ExitCode)
	require.Contains(t, result.ExecutionError.Error(), "exceeds amount")
}

func TestPaymentBrokerUpdateErrorsWhenRedeemingFundsAlreadyRedeemed(t *testing.T) {
	tf.UnitTest(t)

	sys := setup(t)

	// redeem some
	result, err := sys.ApplyRedeemMessage(sys.target, 500, 0)
	require.NoError(t, result.ExecutionError)
	require.NoError(t, err)

	require.Equal(t, uint8(0), result.Receipt.ExitCode)

	// redeeming funds already redeemed is an error
	result, err = sys.ApplyRedeemMessage(sys.target, 400, 1)
	require.NoError(t, err)

	require.NotEqual(t, uint8(0), result.Receipt.ExitCode)
	require.Contains(t, result.ExecutionError.Error(), "update amount")
}

func TestPaymentBrokerUpdateErrorsWhenAtEol(t *testing.T) {
	tf.UnitTest(t)

	sys := setup(t)

	// set block height to Eol
	result, err := sys.ApplyRedeemMessageWithBlockHeight(sys.target, 500, 0, 20000)
	require.NoError(t, err)

	// expect an error
	assert.NotEqual(t, uint8(0), result.Receipt.ExitCode)
	assert.True(t, strings.Contains(strings.ToLower(result.ExecutionError.Error()), "block height"), "Error should relate to block height")
}

func TestPaymentBrokerUpdateErrorsBeforeValidAt(t *testing.T) {
	tf.UnitTest(t)

	sys := setup(t)

	result, err := sys.ApplySignatureMessageWithValidAtAndBlockHeight(sys.target, 100, 0, 8, 3, "redeem")
	require.NoError(t, err)

	assert.NotEqual(t, uint8(0), result.Receipt.ExitCode)
	assert.True(t, strings.Contains(strings.ToLower(result.ExecutionError.Error()), "block height too low"), "Error should relate to height lower than validAt")
}

func TestPaymentBrokerUpdateSuccessWithValidAt(t *testing.T) {
	tf.UnitTest(t)

	sys := setup(t)

	// Redeem at block height == validAt != 0.
	result, err := sys.ApplySignatureMessageWithValidAtAndBlockHeight(sys.target, 100, 0, 4, 4, "redeem")
	require.NoError(t, err)

	require.Equal(t, uint8(0), result.Receipt.ExitCode)

	paymentBroker := state.MustGetActor(sys.st, address.PaymentBrokerAddress)
	assert.Equal(t, types.NewAttoFILFromFIL(900), paymentBroker.Balance)

	payee := state.MustGetActor(sys.st, sys.target)
	assert.Equal(t, types.NewAttoFILFromFIL(100), payee.Balance)

	channel := sys.retrieveChannel(paymentBroker)
	assert.Equal(t, types.NewAttoFILFromFIL(1000), channel.Amount)
	assert.Equal(t, types.NewAttoFILFromFIL(100), channel.AmountRedeemed)
	assert.Equal(t, sys.target, channel.Target)

	// Redeem after block height == validAt.
	result, err = sys.ApplySignatureMessageWithValidAtAndBlockHeight(sys.target, 200, 0, 4, 6, "redeem")
	require.NoError(t, err)

	require.Equal(t, uint8(0), result.Receipt.ExitCode)

	paymentBroker = state.MustGetActor(sys.st, address.PaymentBrokerAddress)
	assert.Equal(t, types.NewAttoFILFromFIL(800), paymentBroker.Balance)

	payee = state.MustGetActor(sys.st, sys.target)
	assert.Equal(t, types.NewAttoFILFromFIL(200), payee.Balance)

	channel = sys.retrieveChannel(paymentBroker)
	assert.Equal(t, types.NewAttoFILFromFIL(1000), channel.Amount)
	assert.Equal(t, types.NewAttoFILFromFIL(200), channel.AmountRedeemed)
	assert.Equal(t, sys.target, channel.Target)
}

func TestPaymentBrokerClose(t *testing.T) {
	tf.UnitTest(t)

	sys := setup(t)

	payerActor := state.MustGetActor(sys.st, sys.payer)
	payerBalancePriorToClose := payerActor.Balance

	result, err := sys.ApplyCloseMessage(sys.target, 100, 0)
	require.NoError(t, err)
	require.NoError(t, result.ExecutionError)

	paymentBroker := state.MustGetActor(sys.st, address.PaymentBrokerAddress)

	// all funds have been redeemed or returned
	assert.Equal(t, types.NewAttoFILFromFIL(0), paymentBroker.Balance)

	targetActor := state.MustGetActor(sys.st, sys.target)

	// targetActor has been paid
	assert.Equal(t, types.NewAttoFILFromFIL(100), targetActor.Balance)

	// remaining balance is returned to payer
	payerActor = state.MustGetActor(sys.st, sys.payer)
	assert.Equal(t, payerBalancePriorToClose.Add(types.NewAttoFILFromFIL(900)), payerActor.Balance)
}

func TestPaymentBrokerCloseErrorsBeforeValidAt(t *testing.T) {
	tf.UnitTest(t)

	sys := setup(t)

	result, err := sys.ApplySignatureMessageWithValidAtAndBlockHeight(sys.target, 100, 0, 8, 3, "close")
	require.NoError(t, err)

	assert.NotEqual(t, uint8(0), result.Receipt.ExitCode)
	assert.True(t, strings.Contains(strings.ToLower(result.ExecutionError.Error()), "block height too low"), "Error should relate to height lower than validAt")
}

func TestPaymentBrokerCloseInvalidSig(t *testing.T) {
	tf.UnitTest(t)

	sys := setup(t)

	amt := types.NewAttoFILFromFIL(100)
	signature, err := sys.Signature(amt, sys.defaultValidAt, nil)
	require.NoError(t, err)
	// make the signature invalid
	signature[0] = 0
	signature[1] = 1

	var condition *types.Predicate
	pdata := core.MustConvertParams(sys.payer, sys.channelID, amt, sys.defaultValidAt, condition, signature, []interface{}{})
	msg := types.NewMessage(sys.target, address.PaymentBrokerAddress, 0, types.NewAttoFILFromFIL(0), "close", pdata)
	res, err := sys.ApplyMessage(msg, 0)
	require.EqualError(t, res.ExecutionError, Errors[ErrInvalidSignature].Error())
	require.NoError(t, err)
}

func TestPaymentBrokerCloseWithCondition(t *testing.T) {
	tf.UnitTest(t)

	addrGetter := address.NewForTestGetter()
	toAddress := addrGetter()

	t.Run("Close should succeed if condition is met", func(t *testing.T) {
		sys := setup(t)
		require.NoError(t, sys.st.SetActor(context.TODO(), toAddress, actor.NewActor(pbTestActorCid, types.ZeroAttoFIL)))

		condition := &types.Predicate{To: toAddress, Method: "paramsNotZero", Params: []interface{}{addrGetter(), uint64(6)}}

		appResult, err := sys.applySignatureMessage(sys.target, 100, types.NewBlockHeight(0), 0, "close", 0, condition, types.NewBlockHeight(43))
		require.NoError(t, err)
		require.NoError(t, appResult.ExecutionError)
	})

	t.Run("Close should fail if condition is _NOT_ met", func(t *testing.T) {
		sys := setup(t)
		require.NoError(t, sys.st.SetActor(context.TODO(), toAddress, actor.NewActor(pbTestActorCid, types.ZeroAttoFIL)))

		condition := &types.Predicate{To: toAddress, Method: "paramsNotZero", Params: []interface{}{address.Undef, uint64(6)}}

		appResult, err := sys.applySignatureMessage(sys.target, 100, types.NewBlockHeight(0), 0, "close", 0, condition, types.NewBlockHeight(43))
		require.NoError(t, err)
		require.Error(t, appResult.ExecutionError)
		require.Contains(t, appResult.ExecutionError.Error(), "failed to validate voucher condition: got undefined address")
	})
}

func TestPaymentBrokerCloseChecksCachedConditions(t *testing.T) {
	tf.UnitTest(t)

	addrGetter := address.NewForTestGetter()
	toAddress := addrGetter()
	method := "paramsNotZero"
	addrParam := addrGetter()
	sectorIdParam := uint64(6)
	payerParams := []interface{}{addrParam, sectorIdParam}
	blockHeightParam := types.NewBlockHeight(43)
	redeemerParams := []interface{}{blockHeightParam}

	sys := setup(t)
	require.NoError(t, sys.st.SetActor(context.TODO(), toAddress, actor.NewActor(pbTestActorCid, types.ZeroAttoFIL)))

	// Close without params and expect a panic
	condition := &types.Predicate{To: toAddress, Method: method, Params: payerParams}
	result, err := sys.applySignatureMessage(sys.target, 100, sys.defaultValidAt, 0, "close", 0, condition)
	require.NoError(t, err)
	require.Error(t, result.ExecutionError)
	require.EqualValues(t, errors.CodeError(result.ExecutionError), ErrConditionInvalid)

	// Successfully redeem the payment channel with params
	condition = &types.Predicate{To: toAddress, Method: method, Params: payerParams}
	result, err = sys.applySignatureMessage(sys.target, 100, types.NewBlockHeight(0), 0, "redeem", 0, condition, redeemerParams...)
	require.NoError(t, err)
	require.NoError(t, result.ExecutionError)

	// Close again without params and expect no error
	result, err = sys.applySignatureMessage(sys.target, 200, sys.defaultValidAt, 0, "close", 0, condition)
	require.NoError(t, err)
	require.NoError(t, result.ExecutionError)
}

func TestPaymentBrokerRedeemInvalidSig(t *testing.T) {
	tf.UnitTest(t)

	sys := setup(t)

	amt := types.NewAttoFILFromFIL(100)
	signature, err := sys.Signature(amt, sys.defaultValidAt, nil)
	require.NoError(t, err)
	// make the signature invalid
	signature[0] = 0
	signature[1] = 1

	var condition *types.Predicate
	pdata := core.MustConvertParams(sys.payer, sys.channelID, amt, sys.defaultValidAt, condition, signature, []interface{}{})
	msg := types.NewMessage(sys.target, address.PaymentBrokerAddress, 0, types.NewAttoFILFromFIL(0), "redeem", pdata)
	res, err := sys.ApplyMessage(msg, 0)
	require.EqualError(t, res.ExecutionError, Errors[ErrInvalidSignature].Error())
	require.NoError(t, err)
}

func TestPaymentBrokerReclaim(t *testing.T) {
	tf.UnitTest(t)

	sys := setup(t)

	payer := state.MustGetActor(sys.st, sys.payer)
	payerBalancePriorToClose := payer.Balance

	pdata := core.MustConvertParams(sys.channelID)
	msg := types.NewMessage(sys.payer, address.PaymentBrokerAddress, 1, types.NewAttoFILFromFIL(0), "reclaim", pdata)
	// block height is after Eol
	res, err := sys.ApplyMessage(msg, 20001)
	require.NoError(t, err)
	require.NoError(t, res.ExecutionError)

	paymentBroker := state.MustGetActor(sys.st, address.PaymentBrokerAddress)

	// all funds have been redeemed or returned
	assert.Equal(t, types.NewAttoFILFromFIL(0), paymentBroker.Balance)

	// entire balance is returned to payer
	payer = state.MustGetActor(sys.st, sys.payer)
	assert.Equal(t, payerBalancePriorToClose.Add(types.NewAttoFILFromFIL(1000)), payer.Balance)
}

func TestPaymentBrokerReclaimFailsBeforeChannelEol(t *testing.T) {
	tf.UnitTest(t)

	sys := setup(t)

	pdata := core.MustConvertParams(sys.channelID)
	msg := types.NewMessage(sys.payer, address.PaymentBrokerAddress, 1, types.NewAttoFILFromFIL(0), "reclaim", pdata)
	// block height is before Eol
	result, err := sys.ApplyMessage(msg, 0)
	require.NoError(t, err)

	// fails
	assert.NotEqual(t, uint8(0), result.Receipt.ExitCode)
	assert.Contains(t, result.ExecutionError.Error(), "reclaim")
	assert.Contains(t, result.ExecutionError.Error(), "eol")
}

func TestPaymentBrokerExtend(t *testing.T) {
	tf.UnitTest(t)

	sys := setup(t)

	// extend channel
	pdata := core.MustConvertParams(sys.channelID, types.NewBlockHeight(30000))
	msg := types.NewMessage(sys.payer, address.PaymentBrokerAddress, 1, types.NewAttoFILFromFIL(1000), "extend", pdata)

	result, err := sys.ApplyMessage(msg, 9)
	require.NoError(t, result.ExecutionError)
	require.NoError(t, err)
	assert.Equal(t, uint8(0), result.Receipt.ExitCode)

	// try to request too high an amount after the eol for the original channel
	result, err = sys.ApplyRedeemMessageWithBlockHeight(sys.target, 1100, 0, 12)
	require.NoError(t, result.ExecutionError)

	// expect success
	require.NoError(t, err)
	assert.Equal(t, uint8(0), result.Receipt.ExitCode)

	// check value
	paymentBroker := state.MustGetActor(sys.st, address.PaymentBrokerAddress)
	assert.Equal(t, types.NewAttoFILFromFIL(900), paymentBroker.Balance) // 1000 + 1000 - 1100

	// get payment channel
	channel := sys.retrieveChannel(paymentBroker)

	assert.Equal(t, types.NewAttoFILFromFIL(2000), channel.Amount)
	assert.Equal(t, types.NewAttoFILFromFIL(1100), channel.AmountRedeemed)
	assert.Equal(t, types.NewBlockHeight(30000), channel.AgreedEol)
	assert.Equal(t, types.NewBlockHeight(30000), channel.Eol)
}

func TestPaymentBrokerExtendFailsWithNonExistentChannel(t *testing.T) {
	tf.UnitTest(t)

	sys := setup(t)

	// extend channel
	pdata := core.MustConvertParams(types.NewChannelID(383), types.NewBlockHeight(30000))
	msg := types.NewMessage(sys.payer, address.PaymentBrokerAddress, 1, types.NewAttoFILFromFIL(1000), "extend", pdata)

	result, err := sys.ApplyMessage(msg, 9)
	require.NoError(t, err)
	require.EqualError(t, result.ExecutionError, "payment channel is unknown")
	assert.NotEqual(t, uint8(0), result.Receipt.ExitCode)
}

func TestPaymentBrokerExtendRefusesToShortenTheEol(t *testing.T) {
	tf.UnitTest(t)

	sys := setup(t)

	// extend channel setting block height to 5 (<10)
	pdata := core.MustConvertParams(sys.channelID, types.NewBlockHeight(5))
	msg := types.NewMessage(sys.payer, address.PaymentBrokerAddress, 1, types.NewAttoFILFromFIL(1000), "extend", pdata)

	result, err := sys.ApplyMessage(msg, 9)
	require.NoError(t, err)

	assert.NotEqual(t, uint8(0), result.Receipt.ExitCode)
	assert.Contains(t, result.ExecutionError.Error(), "payment channel eol may not be decreased")
}

func TestPaymentBrokerCancel(t *testing.T) {
	tf.UnitTest(t)

	sys := setup(t)

	pdata := core.MustConvertParams(sys.channelID)
	msg := types.NewMessage(sys.payer, address.PaymentBrokerAddress, 1, types.NewAttoFILFromFIL(1000), "cancel", pdata)

	result, err := sys.ApplyMessage(msg, 100)
	require.NoError(t, result.ExecutionError)
	require.NoError(t, err)
	assert.Equal(t, uint8(0), result.Receipt.ExitCode)

	paymentBroker := state.MustGetActor(sys.st, address.PaymentBrokerAddress)
	channel := sys.retrieveChannel(paymentBroker)

	assert.Equal(t, types.NewBlockHeight(20000), channel.AgreedEol)
	assert.Equal(t, types.NewBlockHeight(10100), channel.Eol)
}

func TestPaymentBrokerCancelFailsAfterSuccessfulRedeem(t *testing.T) {
	tf.UnitTest(t)

	addrGetter := address.NewForTestGetter()
	toAddress := addrGetter()
	method := "paramsNotZero"
	addrParam := addrGetter()
	sectorIdParam := uint64(6)
	payerParams := []interface{}{addrParam, sectorIdParam}
	blockHeightParam := types.NewBlockHeight(43)
	redeemerParams := []interface{}{blockHeightParam}

	sys := setup(t)
	require.NoError(t, sys.st.SetActor(context.Background(), toAddress, actor.NewActor(pbTestActorCid, types.ZeroAttoFIL)))

	// Successfully redeem the payment channel with params
	condition := &types.Predicate{To: toAddress, Method: method, Params: payerParams}
	result, err := sys.applySignatureMessage(sys.target, 100, types.NewBlockHeight(0), 0, "redeem", 0, condition, redeemerParams...)
	require.NoError(t, err)
	require.NoError(t, result.ExecutionError)

	// Attempts to Cancel and expects failure
	pdata := core.MustConvertParams(sys.channelID)
	msg := types.NewMessage(sys.payer, address.PaymentBrokerAddress, 1, types.NewAttoFILFromFIL(1000), "cancel", pdata)
	result, err = sys.ApplyMessage(msg, 100)
	assert.NoError(t, err)
	assert.Error(t, result.ExecutionError)
	assert.EqualValues(t, ErrInvalidCancel, errors.CodeError(result.ExecutionError))
}

func TestPaymentBrokerCancelFailsAfterSuccessfulRedeemWithNilCondtion(t *testing.T) {
	tf.UnitTest(t)

	addrGetter := address.NewForTestGetter()
	toAddress := addrGetter()

	sys := setup(t)
	require.NoError(t, sys.st.SetActor(context.Background(), toAddress, actor.NewActor(pbTestActorCid, types.ZeroAttoFIL)))

	// Successfully redeem the payment channel with params
	result, err := sys.applySignatureMessage(sys.target, 100, types.NewBlockHeight(0), 0, "redeem", 0, nil)
	require.NoError(t, err)
	require.NoError(t, result.ExecutionError)

	// Attempts to Cancel and expects failure
	pdata := core.MustConvertParams(sys.channelID)
	msg := types.NewMessage(sys.payer, address.PaymentBrokerAddress, 1, types.NewAttoFILFromFIL(1000), "cancel", pdata)
	result, err = sys.ApplyMessage(msg, 100)
	assert.NoError(t, err)
	assert.Error(t, result.ExecutionError)
	assert.EqualValues(t, ErrInvalidCancel, errors.CodeError(result.ExecutionError))
}

func TestPaymentBrokerCancelSucceedsAfterSuccessfulRedeemButFailedConditions(t *testing.T) {
	tf.UnitTest(t)

	addrGetter := address.NewForTestGetter()
	toAddress := addrGetter()
	method := "paramsNotZero"
	sectorIdParam := uint64(6)
	payerParams := []interface{}{toAddress, sectorIdParam}
	blockHeightParam := types.NewBlockHeight(43)
	redeemerParams := []interface{}{blockHeightParam}

	sys := setup(t)
	require.NoError(t, sys.st.SetActor(context.Background(), toAddress, actor.NewActor(pbTestActorCid, types.ZeroAttoFIL)))

	// Successfully redeem the payment channel with params
	condition := &types.Predicate{To: toAddress, Method: method, Params: payerParams}
	result, err := sys.applySignatureMessage(sys.target, 100, types.NewBlockHeight(0), 0, "redeem", 0, condition, redeemerParams...)
	require.NoError(t, err)
	require.NoError(t, result.ExecutionError)

	// Expect that the redeemed flag is true and condition is set
	paymentBroker := state.MustGetActor(sys.st, address.PaymentBrokerAddress)
	channel := sys.retrieveChannel(paymentBroker)
	assert.Equal(t, true, channel.Redeemed)
	assert.NotNil(t, channel.Condition)

	// Change the condition to make it no longer valid
	channel.Condition.Params = []interface{}{}

	// Attempt to Cancel and expects success
	pdata := core.MustConvertParams(sys.channelID)
	msg := types.NewMessage(sys.payer, address.PaymentBrokerAddress, 1, types.NewAttoFILFromFIL(1000), "cancel", pdata)
	_, err = sys.ApplyMessage(msg, 100)
	assert.NoError(t, err)
}

func TestPaymentBrokerLs(t *testing.T) {
	tf.UnitTest(t)

	t.Run("Successfully returns channels", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		payer := address.TestAddress
		target1 := address.NewForTestGetter()()
		target2 := address.NewForTestGetter()()
		_, st, vms := requireGenesis(ctx, t, target1)
		targetActor2 := th.RequireNewAccountActor(t, types.NewAttoFILFromFIL(0))
		st.SetActor(ctx, target2, targetActor2)

		channelID1 := establishChannel(ctx, st, vms, payer, target1, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))
		channelID2 := establishChannel(ctx, st, vms, payer, target2, 1, types.NewAttoFILFromFIL(2000), types.NewBlockHeight(20))

		// retrieve channels
		args, err := abi.ToEncodedValues(payer)
		require.NoError(t, err)

		returnValue, exitCode, err := consensus.CallQueryMethod(ctx, st, vms, address.PaymentBrokerAddress, "ls", args, payer, types.NewBlockHeight(9))
		require.NoError(t, err)
		assert.Equal(t, uint8(0), exitCode)

		channels := make(map[string]*PaymentChannel)
		err = cbor.DecodeInto(returnValue[0], &channels)
		require.NoError(t, err)

		assert.Equal(t, 2, len(channels))

		pc1, found := channels[channelID1.String()]
		require.True(t, found)
		assert.Equal(t, target1, pc1.Target)
		assert.Equal(t, types.NewAttoFILFromFIL(1000), pc1.Amount)
		assert.Equal(t, types.NewAttoFILFromFIL(0), pc1.AmountRedeemed)
		assert.Equal(t, types.NewBlockHeight(10), pc1.AgreedEol)
		assert.Equal(t, types.NewBlockHeight(10), pc1.Eol)

		pc2, found := channels[channelID2.String()]
		require.True(t, found)
		assert.Equal(t, target2, pc2.Target)
		assert.Equal(t, types.NewAttoFILFromFIL(2000), pc2.Amount)
		assert.Equal(t, types.NewAttoFILFromFIL(0), pc2.AmountRedeemed)
		assert.Equal(t, types.NewBlockHeight(20), pc2.AgreedEol)
	})

	t.Run("Returns empty map when payer has no channels", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		payer := address.TestAddress
		target1 := address.NewForTestGetter()()
		_, st, vms := requireGenesis(ctx, t, target1)

		// retrieve channels
		args, err := abi.ToEncodedValues(payer)
		require.NoError(t, err)

		returnValue, exitCode, err := consensus.CallQueryMethod(ctx, st, vms, address.PaymentBrokerAddress, "ls", args, payer, types.NewBlockHeight(9))
		require.NoError(t, err)
		assert.Equal(t, uint8(0), exitCode)

		channels := make(map[string]*PaymentChannel)
		err = cbor.DecodeInto(returnValue[0], &channels)
		require.NoError(t, err)

		assert.Equal(t, 0, len(channels))
	})
}

func TestNewPaymentBrokerVoucher(t *testing.T) {
	tf.UnitTest(t)

	var nilCondition *types.Predicate

	t.Run("Returns valid voucher", func(t *testing.T) {
		sys := setup(t)

		// create voucher
		voucherAmount := types.NewAttoFILFromFIL(100)
		pdata := core.MustConvertParams(sys.channelID, voucherAmount, sys.defaultValidAt, nilCondition)
		msg := types.NewMessage(sys.payer, address.PaymentBrokerAddress, 1, types.ZeroAttoFIL, "voucher", pdata)
		res, err := sys.ApplyMessage(msg, 9)
		assert.NoError(t, err)
		assert.NoError(t, res.ExecutionError)
		assert.Equal(t, uint8(0), res.Receipt.ExitCode)

		voucher := types.PaymentVoucher{}
		err = cbor.DecodeInto(res.Receipt.Return[0], &voucher)
		require.NoError(t, err)

		assert.Equal(t, *sys.channelID, voucher.Channel)
		assert.Equal(t, sys.payer, voucher.Payer)
		assert.Equal(t, sys.target, voucher.Target)
		assert.Equal(t, voucherAmount, voucher.Amount)
		assert.Nil(t, voucher.Condition)
	})

	t.Run("Errors when channel does not exist", func(t *testing.T) {
		sys := setup(t)

		notChannelID := types.NewChannelID(999)

		// create voucher
		voucherAmount := types.NewAttoFILFromFIL(100)
		_, exitCode, err := sys.CallQueryMethod("voucher", 9, notChannelID, voucherAmount, sys.defaultValidAt, nilCondition)
		assert.NotEqual(t, uint8(0), exitCode)
		assert.Contains(t, fmt.Sprintf("%v", err), "unknown")
	})

	t.Run("Errors when voucher exceed channel amount", func(t *testing.T) {
		sys := setup(t)

		// create voucher
		voucherAmount := types.NewAttoFILFromFIL(2000)
		args := core.MustConvertParams(sys.channelID, voucherAmount, sys.defaultValidAt, nilCondition)

		msg := types.NewMessage(sys.payer, address.PaymentBrokerAddress, 1, types.ZeroAttoFIL, "voucher", args)
		res, err := sys.ApplyMessage(msg, 9)
		assert.NoError(t, err)
		assert.NotEqual(t, uint8(0), res.Receipt.ExitCode)
		assert.Contains(t, fmt.Sprintf("%s", res.ExecutionError), "exceeds amount")
	})

	t.Run("Returns valid voucher with condition", func(t *testing.T) {
		sys := setup(t)

		condition := &types.Predicate{
			To:     address.NewForTestGetter()(),
			Method: "someMethod",
			Params: []interface{}{"encoded params"},
		}

		// create voucher
		voucherAmount := types.NewAttoFILFromFIL(100)
		pdata := core.MustConvertParams(sys.channelID, voucherAmount, sys.defaultValidAt, condition)
		msg := types.NewMessage(sys.payer, address.PaymentBrokerAddress, 1, types.ZeroAttoFIL, "voucher", pdata)
		res, err := sys.ApplyMessage(msg, 9)
		assert.NoError(t, err)
		assert.NoError(t, res.ExecutionError)
		assert.Equal(t, uint8(0), res.Receipt.ExitCode)

		voucher := types.PaymentVoucher{}
		err = cbor.DecodeInto(res.Receipt.Return[0], &voucher)
		require.NoError(t, err)

		assert.Equal(t, *sys.channelID, voucher.Channel)
		assert.Equal(t, sys.payer, voucher.Payer)
		assert.Equal(t, sys.target, voucher.Target)
		assert.Equal(t, voucherAmount, voucher.Amount)
	})
}

func TestSignVoucher(t *testing.T) {
	payer := mockSigner.Addresses[0]
	value := types.NewAttoFILFromFIL(10)
	channelId := types.NewChannelID(3)
	blockHeight := types.NewBlockHeight(393)
	condition := &types.Predicate{
		To:     address.NewForTestGetter()(),
		Method: "someMethod",
		Params: []interface{}{"encoded params"},
	}
	var nilCondition *types.Predicate

	t.Run("validates signatures with empty condition", func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)

		sig, err := SignVoucher(channelId, value, blockHeight, payer, nilCondition, mockSigner)
		require.NoError(err)

		assert.True(VerifyVoucherSignature(payer, channelId, value, blockHeight, nilCondition, sig))
		assert.False(VerifyVoucherSignature(payer, channelId, value, blockHeight, condition, sig))
	})

	t.Run("validates signatures with condition", func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)

		sig, err := SignVoucher(channelId, value, blockHeight, payer, condition, mockSigner)
		require.NoError(err)

		assert.True(VerifyVoucherSignature(payer, channelId, value, blockHeight, condition, sig))
		assert.False(VerifyVoucherSignature(payer, channelId, value, blockHeight, nilCondition, sig))
	})
}

func establishChannel(ctx context.Context, st state.Tree, vms vm.StorageMap, from address.Address, target address.Address, nonce uint64, amt types.AttoFIL, eol *types.BlockHeight) *types.ChannelID {
	pdata := core.MustConvertParams(target, eol)
	msg := types.NewMessage(from, address.PaymentBrokerAddress, nonce, amt, "createChannel", pdata)
	result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))
	if err != nil {
		panic(err)
	}

	if result.ExecutionError != nil {
		panic(result.ExecutionError)
	}

	channelID := types.NewChannelIDFromBytes(result.Receipt.Return[0])
	return channelID
}

func requireGenesis(ctx context.Context, t *testing.T, targetAddresses ...address.Address) (*hamt.CborIpldStore, state.Tree, vm.StorageMap) {
	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	vms := vm.NewStorageMap(bs)

	cst := hamt.NewCborStore()
	blk, err := consensus.DefaultGenesis(cst, bs)
	require.NoError(t, err)

	builtinsWithTestActor := map[cid.Cid]exec.ExecutableActor{}
	for cid, actor := range builtin.Actors {
		builtinsWithTestActor[cid] = actor
	}
	builtinsWithTestActor[pbTestActorCid] = &PBTestActor{}

	st, err := state.LoadStateTree(ctx, cst, blk.StateRoot, builtinsWithTestActor)
	require.NoError(t, err)

	for _, addr := range targetAddresses {
		targetActor := th.RequireNewAccountActor(t, types.NewAttoFILFromFIL(0))
		st.SetActor(ctx, addr, targetActor)
	}

	return cst, st, vms
}

// system is a helper struct to allow for easier testing of sending various messages to the paymentbroker actor.
// TODO: could be abstracted to be used in other actor tests.
type system struct {
	t              *testing.T
	ctx            context.Context
	payer          address.Address
	target         address.Address
	defaultValidAt *types.BlockHeight
	channelID      *types.ChannelID
	st             state.Tree
	vms            vm.StorageMap
	addressGetter  func() address.Address
}

func setup(t *testing.T) system {
	t.Helper()

	ctx := context.Background()
	payer := mockSigner.Addresses[0]
	addrGetter := address.NewForTestGetter()
	target := addrGetter()
	defaultValidAt := types.NewBlockHeight(uint64(0))
	_, st, vms := requireGenesis(ctx, t, target)

	payerActor := th.RequireNewAccountActor(t, types.NewAttoFILFromFIL(50000))
	state.MustSetActor(st, payer, payerActor)

	channelID := establishChannel(ctx, st, vms, payer, target, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(20000))

	return system{
		t:              t,
		addressGetter:  addrGetter,
		ctx:            ctx,
		payer:          payer,
		target:         target,
		channelID:      channelID,
		defaultValidAt: defaultValidAt,
		st:             st,
		vms:            vms,
	}
}

func (sys *system) Signature(amt types.AttoFIL, validAt *types.BlockHeight, condition *types.Predicate) ([]byte, error) {
	sig, err := SignVoucher(sys.channelID, amt, validAt, sys.payer, condition, mockSigner)
	if err != nil {
		return nil, err
	}
	return ([]byte)(sig), nil
}

func (sys *system) CallQueryMethod(method string, height uint64, params ...interface{}) ([][]byte, uint8, error) {
	sys.t.Helper()

	args := core.MustConvertParams(params...)

	return consensus.CallQueryMethod(sys.ctx, sys.st, sys.vms, address.PaymentBrokerAddress, method, args, sys.payer, types.NewBlockHeight(height))
}

func (sys *system) ApplyRedeemMessage(target address.Address, amtInt uint64, nonce uint64) (*consensus.ApplicationResult, error) {
	sys.t.Helper()

	return sys.applySignatureMessage(target, amtInt, sys.defaultValidAt, nonce, "redeem", 0, nil)
}

func (sys *system) ApplyRedeemMessageWithBlockHeight(target address.Address, amtInt uint64, nonce uint64, height uint64) (*consensus.ApplicationResult, error) {
	sys.t.Helper()

	return sys.applySignatureMessage(target, amtInt, sys.defaultValidAt, nonce, "redeem", height, nil)
}

func (sys *system) ApplyCloseMessage(target address.Address, amtInt uint64, nonce uint64) (*consensus.ApplicationResult, error) {
	sys.t.Helper()

	return sys.applySignatureMessage(target, amtInt, sys.defaultValidAt, nonce, "close", 0, nil)
}

func (sys *system) ApplySignatureMessageWithValidAtAndBlockHeight(target address.Address, amtInt uint64, nonce uint64, validAt uint64, height uint64, method string) (*consensus.ApplicationResult, error) {
	sys.t.Helper()

	if method != "redeem" && method != "close" {
		sys.t.Fatalf("method %s is not a signature method", method)
	}

	return sys.applySignatureMessage(target, amtInt, types.NewBlockHeight(validAt), nonce, method, height, nil)
}

func (sys *system) retrieveChannel(paymentBroker *actor.Actor) *PaymentChannel {
	// retrieve channels
	args, err := abi.ToEncodedValues(sys.payer)
	require.NoError(sys.t, err)

	returnValue, exitCode, err := consensus.CallQueryMethod(sys.ctx, sys.st, sys.vms, address.PaymentBrokerAddress, "ls", args, sys.payer, types.NewBlockHeight(9))
	require.NoError(sys.t, err)
	assert.Equal(sys.t, uint8(0), exitCode)

	channels := make(map[string]*PaymentChannel)
	err = cbor.DecodeInto(returnValue[0], &channels)
	require.NoError(sys.t, err)

	channel := channels[sys.channelID.KeyString()]
	require.NotNil(sys.t, channel)
	return channel
}

// applySignatureMessage signs voucher parameters and then creates a redeem or close message with all
// the voucher parameters and the signature, sends it to the payment broker, and returns the result
func (sys *system) applySignatureMessage(target address.Address, amtInt uint64, validAt *types.BlockHeight, nonce uint64, method string, height uint64, condition *types.Predicate, suppliedParams ...interface{}) (*consensus.ApplicationResult, error) {
	sys.t.Helper()

	amt := types.NewAttoFILFromFIL(amtInt)
	signature, err := sys.Signature(amt, validAt, condition)
	require.NoError(sys.t, err)

	pdata := core.MustConvertParams(sys.payer, sys.channelID, amt, validAt, condition, signature, suppliedParams)
	msg := types.NewMessage(target, address.PaymentBrokerAddress, nonce, types.NewAttoFILFromFIL(0), method, pdata)

	return sys.ApplyMessage(msg, height)
}

func (sys *system) ApplyMessage(msg *types.Message, height uint64) (*consensus.ApplicationResult, error) {
	return th.ApplyTestMessage(sys.st, sys.vms, msg, types.NewBlockHeight(height))
}

func requireGetPaymentChannel(t *testing.T, ctx context.Context, st state.Tree, vms vm.StorageMap, payer address.Address, channelId *types.ChannelID) *PaymentChannel {
	var paymentMap map[string]*PaymentChannel

	pdata := core.MustConvertParams(payer)
	values, ec, err := consensus.CallQueryMethod(ctx, st, vms, address.PaymentBrokerAddress, "ls", pdata, payer, types.NewBlockHeight(0))
	require.Zero(t, ec)
	require.NoError(t, err)

	actor.UnmarshalStorage(values[0], &paymentMap)

	result, ok := paymentMap[channelId.KeyString()]
	require.True(t, ok)

	return result
}

// PBTestActor is a fake actor for use in tests.
type PBTestActor struct{}

var _ exec.ExecutableActor = (*PBTestActor)(nil)

// Exports returns the list of fake actor exported functions.
func (ma *PBTestActor) Exports() exec.Exports {
	return exec.Exports{
		"paramsNotZero": &exec.FunctionSignature{
			Params: []abi.Type{abi.Address, abi.SectorID, abi.BlockHeight},
			Return: nil,
		},
	}
}

// InitializeState stores this actors
func (ma *PBTestActor) InitializeState(storage exec.Storage, initializerData interface{}) error {
	return nil
}

func (ma *PBTestActor) ParamsNotZero(ctx exec.VMContext, addr address.Address, sector uint64, bh *types.BlockHeight) (uint8, error) {
	if addr == address.Undef {
		return 1, errors.NewRevertError("got undefined address")
	}
	if sector == 0 {
		return 1, errors.NewRevertError("got zero sector")
	}
	if types.NewBlockHeight(0).Equal(bh) {
		return 1, errors.NewRevertError("got zero block height")
	}
	return 0, nil
}
