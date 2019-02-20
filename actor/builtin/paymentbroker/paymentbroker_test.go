package paymentbroker_test

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"gx/ipfs/QmNf3wujpV2Y7Lnj2hy2UrmuX8bhMDStRHbnSLh7Ypf36h/go-hamt-ipld"
	"gx/ipfs/QmRu7tiRnFk9mMPpVECQTBQJqXtmG132jJxA1w9A7TtpBz/go-ipfs-blockstore"
	"gx/ipfs/QmUadX5EcvrBmxAV9sE7wUWtWSqxns5K84qKJBixmcT1w9/go-datastore"
	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	. "github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

var mockSigner, _ = types.NewMockSignersAndKeyInfo(10)

func TestPaymentBrokerGenesis(t *testing.T) {
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, st, _ := requireGenesis(ctx, t, address.NewForTestGetter()())

	paymentBroker := state.MustGetActor(st, address.PaymentBrokerAddress)

	assert.Equal(types.NewAttoFILFromFIL(0), paymentBroker.Balance)
}

func TestPaymentBrokerCreateChannel(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx := context.Background()

	payer := address.TestAddress
	target := address.NewForTestGetter()()
	_, st, vms := requireGenesis(ctx, t, target)

	pdata := core.MustConvertParams(target, big.NewInt(10))
	msg := types.NewMessage(payer, address.PaymentBrokerAddress, 0, types.NewAttoFILFromFIL(1000), "createChannel", pdata)

	result, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))
	require.NoError(err)
	require.NoError(result.ExecutionError)

	st.Flush(ctx)

	channelID := types.NewChannelIDFromBytes(result.Receipt.Return[0])

	paymentBroker := state.MustGetActor(st, address.PaymentBrokerAddress)

	assert.Equal(types.NewAttoFILFromFIL(1000), paymentBroker.Balance)

	channel := requireGetPaymentChannel(t, ctx, st, vms, payer, channelID)

	assert.Equal(types.NewAttoFILFromFIL(1000), channel.Amount)
	assert.Equal(types.NewAttoFILFromFIL(0), channel.AmountRedeemed)
	assert.Equal(target, channel.Target)
	assert.Equal(types.NewBlockHeight(10), channel.Eol)
}

func TestPaymentBrokerUpdate(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	sys := setup(t)

	result, err := sys.ApplyRedeemMessage(sys.target, 100, 0)
	require.NoError(err)
	require.Equal(uint8(0), result.Receipt.ExitCode)

	paymentBroker := state.MustGetActor(sys.st, address.PaymentBrokerAddress)

	assert.Equal(types.NewAttoFILFromFIL(900), paymentBroker.Balance)

	payee := state.MustGetActor(sys.st, sys.target)

	assert.Equal(types.NewAttoFILFromFIL(100), payee.Balance)

	channel := sys.retrieveChannel(paymentBroker)

	assert.Equal(types.NewAttoFILFromFIL(1000), channel.Amount)
	assert.Equal(types.NewAttoFILFromFIL(100), channel.AmountRedeemed)
	assert.Equal(sys.target, channel.Target)
}

func TestPaymentBrokerUpdateErrorsWithIncorrectChannel(t *testing.T) {
	require := require.New(t)
	sys := setup(t)

	// update message from payer instead of target results in error
	result, err := sys.ApplyRedeemMessage(sys.payer, 100, 1)
	require.NoError(err)

	require.NotEqual(uint8(0), result.Receipt.ExitCode)

	// invalid channel id results in revert error
	sys.channelID = types.NewChannelID(39932)
	result, err = sys.ApplyRedeemMessage(sys.target, 100, 0)
	require.NoError(err)

	require.NotEqual(uint8(0), result.Receipt.ExitCode)
	require.Contains(result.ExecutionError.Error(), "unknown")
}

func TestPaymentBrokerUpdateErrorsWhenNotFromTarget(t *testing.T) {
	require := require.New(t)
	sys := setup(t)

	wrongTargetAddress := sys.addressGetter()
	wrongTargetActor := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(0))
	sys.st.SetActor(sys.ctx, wrongTargetAddress, wrongTargetActor)

	result, err := sys.ApplyRedeemMessage(wrongTargetAddress, 100, 0)
	require.NoError(err)

	require.NotEqual(uint8(0), result.Receipt.ExitCode)
	require.Contains(result.ExecutionError.Error(), "wrong target account")
}

func TestPaymentBrokerUpdateErrorsWhenRedeemingMoreThanChannelContains(t *testing.T) {
	require := require.New(t)
	sys := setup(t)

	result, err := sys.ApplyRedeemMessage(sys.target, 1100, 0)
	require.NoError(err)

	require.NotEqual(uint8(0), result.Receipt.ExitCode)
	require.Contains(result.ExecutionError.Error(), "exceeds amount")
}

func TestPaymentBrokerUpdateErrorsWhenRedeemingFundsAlreadyRedeemed(t *testing.T) {
	require := require.New(t)
	sys := setup(t)

	// redeem some
	result, err := sys.ApplyRedeemMessage(sys.target, 500, 0)
	require.NoError(result.ExecutionError)
	require.NoError(err)

	require.Equal(uint8(0), result.Receipt.ExitCode)

	// redeeming funds already redeemed is an error
	result, err = sys.ApplyRedeemMessage(sys.target, 400, 1)
	require.NoError(err)

	require.NotEqual(uint8(0), result.Receipt.ExitCode)
	require.Contains(result.ExecutionError.Error(), "update amount")
}

func TestPaymentBrokerUpdateErrorsWhenAtEol(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	sys := setup(t)

	// set block height to Eol
	result, err := sys.ApplyRedeemMessageWithBlockHeight(sys.target, 500, 0, 10)
	require.NoError(err)

	// expect an error
	assert.NotEqual(uint8(0), result.Receipt.ExitCode)
	assert.True(strings.Contains(strings.ToLower(result.ExecutionError.Error()), "block height"), "Error should relate to block height")
}

func TestPaymentBrokerUpdateErrorsBeforeValidAt(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	sys := setup(t)

	result, err := sys.ApplySignatureMessageWithValidAtAndBlockHeight(sys.target, 100, 0, 8, 3, "redeem")
	require.NoError(err)

	assert.NotEqual(uint8(0), result.Receipt.ExitCode)
	assert.True(strings.Contains(strings.ToLower(result.ExecutionError.Error()), "block height too low"), "Error should relate to height lower than validAt")
}

func TestPaymentBrokerUpdateSuccessWithValidAt(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	sys := setup(t)

	// Redeem at block height == validAt != 0.
	result, err := sys.ApplySignatureMessageWithValidAtAndBlockHeight(sys.target, 100, 0, 4, 4, "redeem")
	require.NoError(err)

	require.Equal(uint8(0), result.Receipt.ExitCode)

	paymentBroker := state.MustGetActor(sys.st, address.PaymentBrokerAddress)
	assert.Equal(types.NewAttoFILFromFIL(900), paymentBroker.Balance)

	payee := state.MustGetActor(sys.st, sys.target)
	assert.Equal(types.NewAttoFILFromFIL(100), payee.Balance)

	channel := sys.retrieveChannel(paymentBroker)
	assert.Equal(types.NewAttoFILFromFIL(1000), channel.Amount)
	assert.Equal(types.NewAttoFILFromFIL(100), channel.AmountRedeemed)
	assert.Equal(sys.target, channel.Target)

	// Redeem after block height == validAt.
	result, err = sys.ApplySignatureMessageWithValidAtAndBlockHeight(sys.target, 200, 0, 4, 6, "redeem")
	require.NoError(err)

	require.Equal(uint8(0), result.Receipt.ExitCode)

	paymentBroker = state.MustGetActor(sys.st, address.PaymentBrokerAddress)
	assert.Equal(types.NewAttoFILFromFIL(800), paymentBroker.Balance)

	payee = state.MustGetActor(sys.st, sys.target)
	assert.Equal(types.NewAttoFILFromFIL(200), payee.Balance)

	channel = sys.retrieveChannel(paymentBroker)
	assert.Equal(types.NewAttoFILFromFIL(1000), channel.Amount)
	assert.Equal(types.NewAttoFILFromFIL(200), channel.AmountRedeemed)
	assert.Equal(sys.target, channel.Target)
}

func TestPaymentBrokerClose(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	sys := setup(t)

	payerActor := state.MustGetActor(sys.st, sys.payer)
	payerBalancePriorToClose := payerActor.Balance

	result, err := sys.ApplyCloseMessage(sys.target, 100, 0)
	require.NoError(err)
	require.NoError(result.ExecutionError)

	paymentBroker := state.MustGetActor(sys.st, address.PaymentBrokerAddress)

	// all funds have been redeemed or returned
	assert.Equal(types.NewAttoFILFromFIL(0), paymentBroker.Balance)

	targetActor := state.MustGetActor(sys.st, sys.target)

	// targetActor has been paid
	assert.Equal(types.NewAttoFILFromFIL(100), targetActor.Balance)

	// remaining balance is returned to payer
	payerActor = state.MustGetActor(sys.st, sys.payer)
	assert.Equal(payerBalancePriorToClose.Add(types.NewAttoFILFromFIL(900)), payerActor.Balance)
}

func TestPaymentBrokerCloseErrorsBeforeValidAt(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	sys := setup(t)

	result, err := sys.ApplySignatureMessageWithValidAtAndBlockHeight(sys.target, 100, 0, 8, 3, "close")
	require.NoError(err)

	assert.NotEqual(uint8(0), result.Receipt.ExitCode)
	assert.True(strings.Contains(strings.ToLower(result.ExecutionError.Error()), "block height too low"), "Error should relate to height lower than validAt")
}

func TestPaymentBrokerCloseInvalidSig(t *testing.T) {
	require := require.New(t)
	sys := setup(t)

	amt := types.NewAttoFILFromFIL(100)
	signature, err := sys.Signature(amt, sys.defaultValidAt)
	require.NoError(err)
	// make the signature invalid
	signature[0] = 0
	signature[1] = 1

	pdata := core.MustConvertParams(sys.payer, sys.channelID, amt, sys.defaultValidAt, signature)
	msg := types.NewMessage(sys.target, address.PaymentBrokerAddress, 0, types.NewAttoFILFromFIL(0), "close", pdata)
	res, err := sys.ApplyMessage(msg, 0)
	require.EqualError(res.ExecutionError, Errors[ErrInvalidSignature].Error())
	require.NoError(err)
}

func TestPaymentBrokerRedeemInvalidSig(t *testing.T) {
	require := require.New(t)
	sys := setup(t)

	amt := types.NewAttoFILFromFIL(100)
	signature, err := sys.Signature(amt, sys.defaultValidAt)
	require.NoError(err)
	// make the signature invalid
	signature[0] = 0
	signature[1] = 1

	pdata := core.MustConvertParams(sys.payer, sys.channelID, amt, sys.defaultValidAt, signature)
	msg := types.NewMessage(sys.target, address.PaymentBrokerAddress, 0, types.NewAttoFILFromFIL(0), "redeem", pdata)
	res, err := sys.ApplyMessage(msg, 0)
	require.EqualError(res.ExecutionError, Errors[ErrInvalidSignature].Error())
	require.NoError(err)
}

func TestPaymentBrokerReclaim(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	sys := setup(t)

	payer := state.MustGetActor(sys.st, sys.payer)
	payerBalancePriorToClose := payer.Balance

	pdata := core.MustConvertParams(sys.channelID)
	msg := types.NewMessage(sys.payer, address.PaymentBrokerAddress, 1, types.NewAttoFILFromFIL(0), "reclaim", pdata)
	// block height is after Eol
	res, err := sys.ApplyMessage(msg, 11)
	require.NoError(err)
	require.NoError(res.ExecutionError)

	paymentBroker := state.MustGetActor(sys.st, address.PaymentBrokerAddress)

	// all funds have been redeemed or returned
	assert.Equal(types.NewAttoFILFromFIL(0), paymentBroker.Balance)

	// entire balance is returned to payer
	payer = state.MustGetActor(sys.st, sys.payer)
	assert.Equal(payerBalancePriorToClose.Add(types.NewAttoFILFromFIL(1000)), payer.Balance)
}

func TestPaymentBrokerReclaimFailsBeforeChannelEol(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	sys := setup(t)

	pdata := core.MustConvertParams(sys.channelID)
	msg := types.NewMessage(sys.payer, address.PaymentBrokerAddress, 1, types.NewAttoFILFromFIL(0), "reclaim", pdata)
	// block height is before Eol
	result, err := sys.ApplyMessage(msg, 0)
	require.NoError(err)

	// fails
	assert.NotEqual(uint8(0), result.Receipt.ExitCode)
	assert.Contains(result.ExecutionError.Error(), "reclaim")
	assert.Contains(result.ExecutionError.Error(), "eol")
}

func TestPaymentBrokerExtend(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	sys := setup(t)

	// extend channel
	pdata := core.MustConvertParams(sys.channelID, types.NewBlockHeight(20))
	msg := types.NewMessage(sys.payer, address.PaymentBrokerAddress, 1, types.NewAttoFILFromFIL(1000), "extend", pdata)

	result, err := sys.ApplyMessage(msg, 9)
	require.NoError(result.ExecutionError)
	require.NoError(err)
	assert.Equal(uint8(0), result.Receipt.ExitCode)

	// try to request too high an amount after the eol for the original channel
	result, err = sys.ApplyRedeemMessageWithBlockHeight(sys.target, 1100, 0, 12)
	require.NoError(result.ExecutionError)

	// expect success
	require.NoError(err)
	assert.Equal(uint8(0), result.Receipt.ExitCode)

	// check value
	paymentBroker := state.MustGetActor(sys.st, address.PaymentBrokerAddress)
	assert.Equal(types.NewAttoFILFromFIL(900), paymentBroker.Balance) // 1000 + 1000 - 1100

	// get payment channel
	channel := sys.retrieveChannel(paymentBroker)

	assert.Equal(types.NewAttoFILFromFIL(2000), channel.Amount)
	assert.Equal(types.NewAttoFILFromFIL(1100), channel.AmountRedeemed)
	assert.Equal(types.NewBlockHeight(20), channel.Eol)
}

func TestPaymentBrokerExtendFailsWithNonExistentChannel(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	sys := setup(t)

	// extend channel
	pdata := core.MustConvertParams(types.NewChannelID(383), types.NewBlockHeight(20))
	msg := types.NewMessage(sys.payer, address.PaymentBrokerAddress, 1, types.NewAttoFILFromFIL(1000), "extend", pdata)

	result, err := sys.ApplyMessage(msg, 9)
	require.NoError(err)
	require.EqualError(result.ExecutionError, "payment channel is unknown")
	assert.NotEqual(uint8(0), result.Receipt.ExitCode)
}

func TestPaymentBrokerExtendRefusesToShortenTheEol(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	sys := setup(t)

	// extend channel setting block height to 5 (<10)
	pdata := core.MustConvertParams(sys.channelID, types.NewBlockHeight(5))
	msg := types.NewMessage(sys.payer, address.PaymentBrokerAddress, 1, types.NewAttoFILFromFIL(1000), "extend", pdata)

	result, err := sys.ApplyMessage(msg, 9)
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
		target1 := address.NewForTestGetter()()
		target2 := address.NewForTestGetter()()
		_, st, vms := requireGenesis(ctx, t, target1)
		targetActor2 := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(0))
		st.SetActor(ctx, target2, targetActor2)

		channelID1 := establishChannel(ctx, st, vms, payer, target1, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))
		channelID2 := establishChannel(ctx, st, vms, payer, target2, 1, types.NewAttoFILFromFIL(2000), types.NewBlockHeight(20))

		// retrieve channels
		args, err := abi.ToEncodedValues(payer)
		require.NoError(err)

		returnValue, exitCode, err := consensus.CallQueryMethod(ctx, st, vms, address.PaymentBrokerAddress, "ls", args, payer, types.NewBlockHeight(9))
		require.NoError(err)
		assert.Equal(uint8(0), exitCode)

		channels := make(map[string]*PaymentChannel)
		err = cbor.DecodeInto(returnValue[0], &channels)
		require.NoError(err)

		assert.Equal(2, len(channels))

		pc1, found := channels[channelID1.String()]
		require.True(found)
		assert.Equal(target1, pc1.Target)
		assert.Equal(types.NewAttoFILFromFIL(1000), pc1.Amount)
		assert.Equal(types.NewAttoFILFromFIL(0), pc1.AmountRedeemed)
		assert.Equal(types.NewBlockHeight(10), pc1.Eol)

		pc2, found := channels[channelID2.String()]
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
		target1 := address.NewForTestGetter()()
		_, st, vms := requireGenesis(ctx, t, target1)

		// retrieve channels
		args, err := abi.ToEncodedValues(payer)
		require.NoError(err)

		returnValue, exitCode, err := consensus.CallQueryMethod(ctx, st, vms, address.PaymentBrokerAddress, "ls", args, payer, types.NewBlockHeight(9))
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
		sys := setup(t)

		// create voucher
		voucherAmount := types.NewAttoFILFromFIL(100)
		pdata := core.MustConvertParams(sys.channelID, voucherAmount, sys.defaultValidAt)
		msg := types.NewMessage(sys.payer, address.PaymentBrokerAddress, 1, nil, "voucher", pdata)
		res, err := sys.ApplyMessage(msg, 9)
		assert.NoError(err)
		assert.NoError(res.ExecutionError)
		assert.Equal(uint8(0), res.Receipt.ExitCode)

		voucher := PaymentVoucher{}
		err = cbor.DecodeInto(res.Receipt.Return[0], &voucher)
		require.NoError(err)

		assert.Equal(*sys.channelID, voucher.Channel)
		assert.Equal(sys.payer, voucher.Payer)
		assert.Equal(sys.target, voucher.Target)
		assert.Equal(*voucherAmount, voucher.Amount)
	})

	t.Run("Errors when channel does not exist", func(t *testing.T) {
		sys := setup(t)

		notChannelID := types.NewChannelID(999)

		// create voucher
		voucherAmount := types.NewAttoFILFromFIL(100)
		_, exitCode, err := sys.CallQueryMethod("voucher", 9, notChannelID, voucherAmount, sys.defaultValidAt)
		assert.NotEqual(uint8(0), exitCode)
		assert.Contains(fmt.Sprintf("%v", err), "unknown")
	})

	t.Run("Errors when voucher exceed channel amount", func(t *testing.T) {
		sys := setup(t)

		// create voucher
		voucherAmount := types.NewAttoFILFromFIL(2000)
		args := core.MustConvertParams(sys.channelID, voucherAmount, sys.defaultValidAt)

		msg := types.NewMessage(sys.payer, address.PaymentBrokerAddress, 1, nil, "voucher", args)
		res, err := sys.ApplyMessage(msg, 9)
		assert.NoError(err)
		assert.NotEqual(uint8(0), res.Receipt.ExitCode)
		assert.Contains(fmt.Sprintf("%s", res.ExecutionError), "exceeds amount")
	})
}

func establishChannel(ctx context.Context, st state.Tree, vms vm.StorageMap, from address.Address, target address.Address, nonce uint64, amt *types.AttoFIL, eol *types.BlockHeight) *types.ChannelID {
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
	require := require.New(t)

	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	vms := vm.NewStorageMap(bs)

	cst := hamt.NewCborStore()
	blk, err := consensus.DefaultGenesis(cst, bs)
	require.NoError(err)

	st, err := state.LoadStateTree(ctx, cst, blk.StateRoot, builtin.Actors)
	require.NoError(err)

	for _, addr := range targetAddresses {
		targetActor := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(0))
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

	payerActor := th.RequireNewAccountActor(require.New(t), types.NewAttoFILFromFIL(50000))
	state.MustSetActor(st, payer, payerActor)

	channelID := establishChannel(ctx, st, vms, payer, target, 0, types.NewAttoFILFromFIL(1000), types.NewBlockHeight(10))

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

func (sys *system) Signature(amt *types.AttoFIL, validAt *types.BlockHeight) ([]byte, error) {
	sig, err := SignVoucher(sys.channelID, amt, validAt, sys.payer, mockSigner)
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

	return sys.applySignatureMessage(target, amtInt, sys.defaultValidAt, nonce, "redeem", 0)
}

func (sys *system) ApplyRedeemMessageWithBlockHeight(target address.Address, amtInt uint64, nonce uint64, height uint64) (*consensus.ApplicationResult, error) {
	sys.t.Helper()

	return sys.applySignatureMessage(target, amtInt, sys.defaultValidAt, nonce, "redeem", height)
}

func (sys *system) ApplyCloseMessage(target address.Address, amtInt uint64, nonce uint64) (*consensus.ApplicationResult, error) {
	sys.t.Helper()

	return sys.applySignatureMessage(target, amtInt, sys.defaultValidAt, nonce, "close", 0)
}

func (sys *system) ApplySignatureMessageWithValidAtAndBlockHeight(target address.Address, amtInt uint64, nonce uint64, validAt uint64, height uint64, method string) (*consensus.ApplicationResult, error) {
	sys.t.Helper()

	if method != "redeem" && method != "close" {
		sys.t.Fatalf("method %s is not a signature method", method)
	}

	return sys.applySignatureMessage(target, amtInt, types.NewBlockHeight(validAt), nonce, method, height)
}

func (sys *system) retrieveChannel(paymentBroker *actor.Actor) *PaymentChannel {
	assert := assert.New(sys.t)
	require := require.New(sys.t)

	// retrieve channels
	args, err := abi.ToEncodedValues(sys.payer)
	require.NoError(err)

	returnValue, exitCode, err := consensus.CallQueryMethod(sys.ctx, sys.st, sys.vms, address.PaymentBrokerAddress, "ls", args, sys.payer, types.NewBlockHeight(9))
	require.NoError(err)
	assert.Equal(uint8(0), exitCode)

	channels := make(map[string]*PaymentChannel)
	err = cbor.DecodeInto(returnValue[0], &channels)
	require.NoError(err)

	channel := channels[sys.channelID.KeyString()]
	require.NotNil(channel)
	return channel
}

func (sys *system) applySignatureMessage(target address.Address, amtInt uint64, validAt *types.BlockHeight, nonce uint64, method string, height uint64) (*consensus.ApplicationResult, error) {
	sys.t.Helper()

	require := require.New(sys.t)

	amt := types.NewAttoFILFromFIL(amtInt)
	signature, err := sys.Signature(amt, validAt)
	require.NoError(err)

	pdata := core.MustConvertParams(sys.payer, sys.channelID, amt, validAt, signature)
	msg := types.NewMessage(target, address.PaymentBrokerAddress, nonce, types.NewAttoFILFromFIL(0), method, pdata)

	return sys.ApplyMessage(msg, height)
}

func (sys *system) ApplyMessage(msg *types.Message, height uint64) (*consensus.ApplicationResult, error) {
	return th.ApplyTestMessage(sys.st, sys.vms, msg, types.NewBlockHeight(height))
}

func requireGetPaymentChannel(t *testing.T, ctx context.Context, st state.Tree, vms vm.StorageMap, payer address.Address, channelId *types.ChannelID) *PaymentChannel {
	require := require.New(t)
	var paymentMap map[string]*PaymentChannel

	pdata := core.MustConvertParams(payer)
	values, ec, err := consensus.CallQueryMethod(ctx, st, vms, address.PaymentBrokerAddress, "ls", pdata, payer, types.NewBlockHeight(0))
	require.Zero(ec)
	require.NoError(err)

	actor.UnmarshalStorage(values[0], &paymentMap)

	result, ok := paymentMap[channelId.KeyString()]
	require.True(ok)

	return result
}
