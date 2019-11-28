package paymentbroker

import (
	"context"
	"reflect"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	internal "github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/pattern"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/storage"
)

// Actor provides a mechanism for off chain payments.
// It allows the creation of payment channels that hold funds for a target account
// and permits that account to withdraw funds only with a voucher signed by the
// channel's creator.
type Actor struct{}

// PaymentChannel records the intent to pay funds to a target account.
type PaymentChannel struct {
	// Target is the address of the account to which funds will be transferred
	Target address.Address `json:"target"`

	// Amount is the total amount of FIL that has been transferred to the channel from the payer
	Amount types.AttoFIL `json:"amount"`

	// AmountRedeemed is the amount of FIL already transferred to the target
	AmountRedeemed types.AttoFIL `json:"amount_redeemed"`

	// AgreedEol is the expiration for the payment channel agreed upon by the
	// payer and payee upon initialization or extension
	AgreedEol *types.BlockHeight `json:"agreed_eol"`

	// Condition are the set of conditions for redeeming or closing the payment
	// channel
	Condition *types.Predicate `json:"condition"`

	// Eol is the actual expiration for the payment channel which can differ from
	// AgreedEol when the payment channel is in dispute
	Eol *types.BlockHeight `json:"eol"`

	// Redeemed is a flag indicating whether or not Redeem has been called on the
	// payment channel yet. This is necessary because AmountRedeemed can still be
	// zero in the event of a zero-value voucher
	Redeemed bool `json:"redeemed"`
}

// Actor methods
const (
	Cancel types.MethodID = iota + 32
	Close
	CreateChannel
	Extend
	Ls
	Reclaim
	Redeem
	Voucher
)

// NewActor returns a new payment broker actor.
func NewActor() *actor.Actor {
	return actor.NewActor(types.PaymentBrokerActorCodeCid, types.ZeroAttoFIL)
}

//
// ExecutableActor impl for Actor
//

var _ dispatch.ExecutableActor = (*Actor)(nil)

var signatures = dispatch.Exports{
	Cancel: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.ChannelID},
		Return: nil,
	},
	Close: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.Address, abi.ChannelID, abi.AttoFIL, abi.BlockHeight, abi.Predicate, abi.Bytes, abi.Parameters},
		Return: nil,
	},
	CreateChannel: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.Address, abi.BlockHeight},
		Return: []abi.Type{abi.ChannelID},
	},
	Extend: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.ChannelID, abi.BlockHeight},
		Return: nil,
	},
	Ls: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.Address},
		Return: []abi.Type{abi.Bytes},
	},
	Reclaim: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.ChannelID},
		Return: nil,
	},
	Redeem: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.Address, abi.ChannelID, abi.AttoFIL, abi.BlockHeight, abi.Predicate, abi.Bytes, abi.Parameters},
		Return: nil,
	},
	Voucher: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.ChannelID, abi.AttoFIL, abi.BlockHeight, abi.Predicate},
		Return: []abi.Type{abi.Bytes},
	},
}

// Method returns method definition for a given method id.
func (a *Actor) Method(id types.MethodID) (dispatch.Method, *dispatch.FunctionSignature, bool) {
	switch id {
	case Cancel:
		return reflect.ValueOf((*impl)(a).cancel), signatures[Cancel], true
	case Close:
		return reflect.ValueOf((*impl)(a).close), signatures[Close], true
	case CreateChannel:
		return reflect.ValueOf((*impl)(a).createChannel), signatures[CreateChannel], true
	case Extend:
		return reflect.ValueOf((*impl)(a).extend), signatures[Extend], true
	case Ls:
		return reflect.ValueOf((*impl)(a).ls), signatures[Ls], true
	case Reclaim:
		return reflect.ValueOf((*impl)(a).reclaim), signatures[Reclaim], true
	case Redeem:
		return reflect.ValueOf((*impl)(a).redeem), signatures[Redeem], true
	case Voucher:
		return reflect.ValueOf((*impl)(a).voucher), signatures[Voucher], true
	default:
		return nil, nil, false
	}
}

// InitializeState stores the actor's initial data structure.
func (*Actor) InitializeState(storage runtime.Storage, initializerData interface{}) error {
	// pb's default state is an empty lookup, so this method is a no-op
	return nil
}

//
// vm methods for actor
//

type impl Actor

const (
	// ErrNonAccountActor indicates an non-account actor attempted to create a payment channel.
	ErrNonAccountActor = 33
	// ErrDuplicateChannel indicates an attempt to create a payment channel with an existing id.
	ErrDuplicateChannel = 34
	// ErrEolTooLow indicates an attempt to lower the Eol of a payment channel.
	ErrEolTooLow = 35
	// ErrReclaimBeforeEol indicates an attempt to reclaim funds before the eol of the channel.
	ErrReclaimBeforeEol = 36
	// ErrInsufficientChannelFunds indicates an attempt to take more funds than the channel contains.
	ErrInsufficientChannelFunds = 37
	// ErrUnknownChannel indicates an invalid channel id.
	ErrUnknownChannel = 38
	// ErrWrongTarget indicates attempt to redeem from wrong target account.
	ErrWrongTarget = 39
	// ErrExpired indicates the block height has exceeded the eol.
	ErrExpired = 40
	// ErrAlreadyWithdrawn indicates amount of the voucher has already been withdrawn.
	ErrAlreadyWithdrawn = 41
	// ErrInvalidSignature indicates the signature is invalid.
	ErrInvalidSignature = 42
	//ErrTooEarly indicates that the block height is too low to satisfy a voucher
	ErrTooEarly = 43
	//ErrConditionInvalid indicates that the condition attached to a voucher did not execute successfully
	ErrConditionInvalid = 44
	//ErrInvalidCancel indicates that the condition attached to a voucher did execute successfully and therefore can't be cancelled
	ErrInvalidCancel = 45
)

// CancelDelayBlockTime is the number of rounds given to the target to respond after the channel
// is canceled before it expires.
// TODO: what is a secure value for this?  Value is arbitrary right now.
// See https://github.com/filecoin-project/go-filecoin/issues/1887
const CancelDelayBlockTime = 10000

// Errors map error codes to revert errors this actor may return.
var Errors = map[uint8]error{
	ErrTooEarly:                 errors.NewCodedRevertError(ErrTooEarly, "block height too low to redeem voucher"),
	ErrNonAccountActor:          errors.NewCodedRevertError(ErrNonAccountActor, "Only account actors may create payment channels"),
	ErrDuplicateChannel:         errors.NewCodedRevertError(ErrDuplicateChannel, "Duplicate create channel attempt"),
	ErrEolTooLow:                errors.NewCodedRevertError(ErrEolTooLow, "payment channel eol may not be decreased"),
	ErrReclaimBeforeEol:         errors.NewCodedRevertError(ErrReclaimBeforeEol, "payment channel may not be reclaimed before eol"),
	ErrInsufficientChannelFunds: errors.NewCodedRevertError(ErrInsufficientChannelFunds, "voucher amount exceeds amount in channel"),
	ErrUnknownChannel:           errors.NewCodedRevertError(ErrUnknownChannel, "payment channel is unknown"),
	ErrWrongTarget:              errors.NewCodedRevertError(ErrWrongTarget, "attempt to redeem channel from wrong target account"),
	ErrExpired:                  errors.NewCodedRevertError(ErrExpired, "block height has exceeded channel's end of life"),
	ErrAlreadyWithdrawn:         errors.NewCodedRevertError(ErrAlreadyWithdrawn, "update amount has already been redeemed"),
	ErrInvalidSignature:         errors.NewCodedRevertErrorf(ErrInvalidSignature, "signature failed to validate"),
}

// Dragons: verify if the message is still needed
type invocationContext interface {
	runtime.InvocationContext
	LegacyMessage() *types.UnsignedMessage
}

// CreateChannel creates a new payment channel from the caller to the target.
// The value attached to the invocation is used as the deposit, and the channel
// will expire and return all of its money to the owner after the given block height.
func (*impl) createChannel(vmctx invocationContext, target address.Address, eol *types.BlockHeight) (*types.ChannelID, uint8, error) {
	if err := vmctx.Charge(actor.DefaultGasCost); err != nil {
		return nil, internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	// require that from account be an account actor to ensure nonce is a valid id
	vmctx.ValidateCaller(pattern.IsAccountActor{})

	ctx := context.Background()
	st := vmctx.Runtime().Storage()
	payerAddress := vmctx.Message().Caller()
	channelID := types.NewChannelID(uint64(vmctx.LegacyMessage().CallSeqNum))

	err := withPayerChannels(ctx, st, payerAddress, func(byChannelID storage.Lookup) error {
		// check to see if payment channel is duplicate
		err := byChannelID.Find(ctx, channelID.KeyString(), nil)
		if err != hamt.ErrNotFound { // we expect to not find the payment channel
			if err == nil {
				return Errors[ErrDuplicateChannel]
			}
			return errors.FaultErrorWrapf(err, "Error retrieving payment channel")
		}

		// add payment channel and commit
		err = byChannelID.Set(ctx, channelID.KeyString(), &PaymentChannel{
			Target:         target,
			Amount:         vmctx.Message().ValueReceived(),
			AmountRedeemed: types.NewAttoFILFromFIL(0),
			AgreedEol:      eol,
			Eol:            eol,
		})
		if err != nil {
			return errors.FaultErrorWrap(err, "Could not set payment channel")
		}

		return nil
	})

	if err != nil {
		// ensure error is properly wrapped
		if !errors.IsFault(err) && !errors.ShouldRevert(err) {
			return nil, 1, errors.FaultErrorWrap(err, "Error creating payment channel")
		}
		return nil, errors.CodeError(err), err
	}

	return channelID, 0, nil
}

// Redeem is called by the target account to withdraw funds with authorization from the payer.
// This method is exactly like Close except it doesn't close the channel.
// This is useful when you want to checkpoint the value in a payment, but continue to use the
// channel afterwards. The amt represents the total funds authorized so far, so that subsequent
// calls to Update will only transfer the difference between the given amt and the greatest
// amt taken so far. A series of channel transactions might look like this:
//                                Payer: 2000, Target: 0, Channel: 0
// payer createChannel(1000)   -> Payer: 1000, Target: 0, Channel: 1000
// target Redeem(100)          -> Payer: 1000, Target: 100, Channel: 900
// target Redeem(200)          -> Payer: 1000, Target: 200, Channel: 800
// target Close(500)           -> Payer: 1500, Target: 500, Channel: 0
//
// If a condition is provided in the voucher:
// - The parameters provided in the condition will be combined with redeemerConditionParams
// - A message will be sent to the the condition.To address using the condition.Method with the combined params
// - If the message returns an error the condition is considered to be false and the redeem will fail
func (*impl) redeem(vmctx invocationContext, payer address.Address, chid *types.ChannelID, amt types.AttoFIL,
	validAt *types.BlockHeight, condition *types.Predicate, sig []byte, redeemerConditionParams []interface{}) (uint8, error) {

	if err := vmctx.Charge(actor.DefaultGasCost); err != nil {
		return internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	if !VerifyVoucherSignature(payer, chid, amt, validAt, condition, sig) {
		return errors.CodeError(Errors[ErrInvalidSignature]), Errors[ErrInvalidSignature]
	}

	ctx := context.Background()
	st := vmctx.Runtime().Storage()

	err := withPayerChannels(ctx, st, payer, func(byChannelID storage.Lookup) error {
		var channel PaymentChannel
		err := byChannelID.Find(ctx, chid.KeyString(), &channel)
		if err != nil {
			if err == hamt.ErrNotFound {
				return Errors[ErrUnknownChannel]
			}
			return errors.FaultErrorWrapf(err, "Could not retrieve payment channel with ID: %s", chid)
		}

		// validate the amount can be sent to the target and send payment to that address.
		err = validateAndUpdateChannel(vmctx, vmctx.Message().Caller(), &channel, amt, validAt, condition, redeemerConditionParams)
		if err != nil {
			return err
		}

		// Reset the EOL to the originally agreed upon EOL in the event that the
		// channel has been cancelled.
		channel.Eol = channel.AgreedEol

		// Mark the payment channel as redeemed
		channel.Redeemed = true

		return byChannelID.Set(ctx, chid.KeyString(), channel)
	})

	if err != nil {
		// ensure error is properly wrapped
		if !errors.IsFault(err) && !errors.ShouldRevert(err) {
			return 1, errors.FaultErrorWrap(err, "Error redeeming payment channel")
		}
		return errors.CodeError(err), err
	}

	return 0, nil
}

// Close first executes the logic performed in the the Update method, then returns all
// funds remaining in the channel to the payer account and deletes the channel.
//
// If a condition is provided in the voucher:
// - The parameters provided in the condition will be combined with redeemerConditionParams
// - A message will be sent to the the condition.To address using the condition.Method with the combined params
// - If the message returns an error the condition is considered to be false and the redeem will fail
func (*impl) close(vmctx invocationContext, payer address.Address, chid *types.ChannelID, amt types.AttoFIL,
	validAt *types.BlockHeight, condition *types.Predicate, sig []byte, redeemerConditionParams []interface{}) (uint8, error) {

	if err := vmctx.Charge(actor.DefaultGasCost); err != nil {
		return internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	if !VerifyVoucherSignature(payer, chid, amt, validAt, condition, sig) {
		return errors.CodeError(Errors[ErrInvalidSignature]), Errors[ErrInvalidSignature]
	}

	ctx := context.Background()
	st := vmctx.Runtime().Storage()

	err := withPayerChannels(ctx, st, payer, func(byChannelID storage.Lookup) error {
		var channel PaymentChannel
		err := byChannelID.Find(ctx, chid.KeyString(), &channel)
		if err != nil {
			if err == hamt.ErrNotFound {
				return Errors[ErrUnknownChannel]
			}
			return errors.FaultErrorWrapf(err, "Could not retrieve payment channel with ID: %s", chid)
		}

		// validate the amount can be sent to the target and send payment to that address.
		err = validateAndUpdateChannel(vmctx, vmctx.Message().Caller(), &channel, amt, validAt, condition, redeemerConditionParams)
		if err != nil {
			return err
		}

		err = byChannelID.Set(ctx, chid.KeyString(), channel)
		if err != nil {
			return err
		}

		// return funds to payer
		return reclaim(ctx, vmctx, byChannelID, payer, chid, &channel)
	})

	if err != nil {
		// ensure error is properly wrapped
		if !errors.IsFault(err) && !errors.ShouldRevert(err) {
			return 1, errors.FaultErrorWrap(err, "Error updating or reclaiming channel")
		}
		return errors.CodeError(err), err
	}

	return 0, nil
}

// Extend can be used by the owner of a channel to add more funds to it and
// extend the Channel's lifespan.
func (*impl) extend(vmctx invocationContext, chid *types.ChannelID, eol *types.BlockHeight) (uint8, error) {
	if err := vmctx.Charge(actor.DefaultGasCost); err != nil {
		return internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	ctx := context.Background()
	st := vmctx.Runtime().Storage()
	payerAddress := vmctx.Message().Caller()

	err := withPayerChannels(ctx, st, payerAddress, func(byChannelID storage.Lookup) error {
		var channel PaymentChannel
		err := byChannelID.Find(ctx, chid.KeyString(), &channel)
		if err != nil {
			if err == hamt.ErrNotFound {
				return Errors[ErrUnknownChannel]
			}
			return errors.FaultErrorWrapf(err, "Could not retrieve payment channel with ID: %s", chid)
		}

		// eol can only be increased
		if channel.Eol.GreaterThan(eol) {
			return Errors[ErrEolTooLow]
		}

		// set new eol
		channel.AgreedEol = eol
		channel.Eol = eol

		// increment the value
		channel.Amount = channel.Amount.Add(vmctx.Message().ValueReceived())

		return byChannelID.Set(ctx, chid.KeyString(), channel)
	})

	if err != nil {
		// ensure error is properly wrapped
		if !errors.IsFault(err) && !errors.ShouldRevert(err) {
			return 1, errors.FaultErrorWrap(err, "Error extending channel")
		}
		return errors.CodeError(err), err
	}

	return 0, nil
}

// Cancel can be used to end an off chain payment early. It lowers the EOL of
// the payment channel to 1 blocktime from now and allows a caller to reclaim
// their payments. In the time before the channel is closed, a target can
// potentially dispute a closer. Cancel will only succeed if the target has not
// successfully redeemed a voucher or if the target has successfully redeemed
// the channel with a conditional voucher and the condition is no longer valid
// due to changes in chain state.
func (*impl) cancel(vmctx invocationContext, chid *types.ChannelID) (uint8, error) {
	if err := vmctx.Charge(actor.DefaultGasCost); err != nil {
		return internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	ctx := context.Background()
	st := vmctx.Runtime().Storage()
	payerAddress := vmctx.Message().Caller()

	err := withPayerChannels(ctx, st, payerAddress, func(byChannelID storage.Lookup) error {
		var channel PaymentChannel
		err := byChannelID.Find(ctx, chid.KeyString(), &channel)
		if err != nil {
			if err == hamt.ErrNotFound {
				return Errors[ErrUnknownChannel]
			}
			return errors.FaultErrorWrapf(err, "Could not retrieve payment channel with ID: %s", chid)
		}

		// Check if channel has already been redeemed and re-run condition if necessary
		if channel.Redeemed {
			// If it doesn't have a condition, it's valid, so throw an error
			if channel.Condition == nil {
				return errors.NewCodedRevertError(ErrInvalidCancel, "channel cannot be cancelled due to successful redeem")
			}
			// Otherwise, check the condition on the payment channel
			err := checkCondition(vmctx, &channel)
			// If we receive no error, the condition is valid, so we fail
			if err == nil {
				return errors.NewCodedRevertError(ErrInvalidCancel, "channel cannot be cancelled due to successful redeem")
			}
			// If there's a non-revert error, we have bigger problem, so raise the
			// error
			if !errors.ShouldRevert(err) {
				return err
			}
		}

		epoch := vmctx.Runtime().CurrentEpoch()
		eol := epoch.Add(types.NewBlockHeight(CancelDelayBlockTime))

		// eol can only be decreased
		if channel.Eol.GreaterThan(eol) {
			channel.Eol = eol
		}

		return byChannelID.Set(ctx, chid.KeyString(), channel)
	})

	if err != nil {
		// ensure error is properly wrapped
		if !errors.IsFault(err) && !errors.ShouldRevert(err) {
			return 1, errors.FaultErrorWrap(err, "Error cancelling channel")
		}
		return errors.CodeError(err), err
	}

	return 0, nil
}

// Reclaim is used by the owner of a channel to reclaim unspent funds in timed
// out payment Channels they own.
func (*impl) reclaim(vmctx invocationContext, chid *types.ChannelID) (uint8, error) {
	if err := vmctx.Charge(actor.DefaultGasCost); err != nil {
		return internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	ctx := context.Background()
	st := vmctx.Runtime().Storage()
	payerAddress := vmctx.Message().Caller()

	err := withPayerChannels(ctx, st, payerAddress, func(byChannelID storage.Lookup) error {
		var channel PaymentChannel
		err := byChannelID.Find(ctx, chid.KeyString(), &channel)
		if err != nil {
			if err == hamt.ErrNotFound {
				return Errors[ErrUnknownChannel]
			}
			return errors.FaultErrorWrapf(err, "Could not retrieve payment channel with ID: %s", chid)
		}

		// reclaim may only be called at or after Eol
		epoch := vmctx.Runtime().CurrentEpoch()
		if epoch.LessThan(channel.Eol) {
			return Errors[ErrReclaimBeforeEol]
		}

		// return funds to payer
		return reclaim(ctx, vmctx, byChannelID, payerAddress, chid, &channel)
	})

	if err != nil {
		// ensure error is properly wrapped
		if !errors.IsFault(err) && !errors.ShouldRevert(err) {
			return 1, errors.FaultErrorWrap(err, "Error reclaiming channel")
		}
		return errors.CodeError(err), err
	}

	return 0, nil
}

// Voucher takes a channel id and amount creates a new unsigned PaymentVoucher
// against the given channel.  It also takes a block height parameter "validAt"
// enforcing that the voucher is not reclaimed until the given block height
// Voucher errors if the channel doesn't exist or contains less than request
// amount.
// If a condition is provided, attempts to redeem or close with the voucher will
// first send a message based on the condition and require a successful response
// for funds to be transferred.
func (*impl) voucher(vmctx invocationContext, chid *types.ChannelID, amount types.AttoFIL, validAt *types.BlockHeight, condition *types.Predicate) ([]byte, uint8, error) {
	if err := vmctx.Charge(actor.DefaultGasCost); err != nil {
		return []byte{}, internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	ctx := context.Background()
	st := vmctx.Runtime().Storage()
	payerAddress := vmctx.Message().Caller()
	var voucher types.PaymentVoucher

	err := withPayerChannelsForReading(ctx, st, payerAddress, func(byChannelID storage.Lookup) error {
		var channel PaymentChannel
		err := byChannelID.Find(ctx, chid.KeyString(), &channel)
		if err != nil {
			if err == hamt.ErrNotFound {
				return Errors[ErrUnknownChannel]
			}
			return errors.FaultErrorWrapf(err, "Could not retrieve payment channel with ID: %s", chid)
		}

		// voucher must be for less than total amount in channel
		if channel.Amount.LessThan(amount) {
			return Errors[ErrInsufficientChannelFunds]
		}

		// set voucher
		voucher = types.PaymentVoucher{
			Channel:   *chid,
			Payer:     vmctx.Message().Caller(),
			Target:    channel.Target,
			Amount:    amount,
			ValidAt:   *validAt,
			Condition: condition,
		}

		return nil
	})

	if err != nil {
		// ensure error is properly wrapped
		if !errors.IsFault(err) && !errors.ShouldRevert(err) {
			return nil, 1, errors.FaultErrorWrap(err, "Error reclaiming channel")
		}
		return nil, errors.CodeError(err), err
	}

	voucherBytes, err := actor.MarshalStorage(voucher)
	if err != nil {
		return nil, 1, errors.FaultErrorWrap(err, "Error marshalling voucher")
	}

	return voucherBytes, 0, nil
}

// Ls returns all payment channels for a given payer address.
// The slice of channels will be returned as cbor encoded map from string channelId to PaymentChannel.
func (*impl) ls(vmctx invocationContext, payer address.Address) ([]byte, uint8, error) {
	if err := vmctx.Charge(actor.DefaultGasCost); err != nil {
		return []byte{}, internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	ctx := context.Background()
	st := vmctx.Runtime().Storage()
	channels := map[string]*PaymentChannel{}

	err := withPayerChannelsForReading(ctx, st, payer, func(byChannelID storage.Lookup) error {
		return byChannelID.ForEachValue(ctx, &PaymentChannel{}, func(k string, value interface{}) error {
			pc, ok := value.(*PaymentChannel)
			if !ok {
				return errors.NewFaultError("Expected PaymentChannel from channel lookup")
			}
			channels[k] = pc
			return nil
		})
	})

	if err != nil {
		// ensure error is properly wrapped
		if !errors.IsFault(err) && !errors.ShouldRevert(err) {
			return nil, 1, errors.FaultErrorWrap(err, "Error reclaiming channel")
		}
		return nil, errors.CodeError(err), err
	}

	channelsBytes, err := actor.MarshalStorage(channels)
	if err != nil {
		return nil, 1, errors.FaultErrorWrap(err, "Error marshalling voucher")
	}

	return channelsBytes, 0, nil
}

func validateAndUpdateChannel(ctx invocationContext, target address.Address, channel *PaymentChannel, amt types.AttoFIL, validAt *types.BlockHeight, condition *types.Predicate, redeemerSuppliedParams []interface{}) error {
	cacheCondition(channel, condition, redeemerSuppliedParams)

	if err := checkCondition(ctx, channel); err != nil {
		return err
	}

	if target != channel.Target {
		return Errors[ErrWrongTarget]
	}

	epoch := ctx.Runtime().CurrentEpoch()
	if epoch.LessThan(validAt) {
		return Errors[ErrTooEarly]
	}

	if epoch.GreaterEqual(channel.Eol) {
		return Errors[ErrExpired]
	}

	if amt.GreaterThan(channel.Amount) {
		return Errors[ErrInsufficientChannelFunds]
	}

	if amt.LessEqual(channel.AmountRedeemed) {
		return Errors[ErrAlreadyWithdrawn]
	}

	// transfer funds to sender
	updateAmount := amt.Sub(channel.AmountRedeemed)
	_, _, err := ctx.Runtime().Send(ctx.Message().Caller(), types.SendMethodID, updateAmount, nil)
	if err != nil {
		return err
	}

	// update amount redeemed from this channel
	channel.AmountRedeemed = amt

	return nil
}

func reclaim(ctx context.Context, vmctx invocationContext, byChannelID storage.Lookup, payer address.Address, chid *types.ChannelID, channel *PaymentChannel) error {
	amt := channel.Amount.Sub(channel.AmountRedeemed)
	if amt.LessEqual(types.ZeroAttoFIL) {
		return nil
	}

	// clean up
	err := byChannelID.Delete(ctx, chid.KeyString())
	if err != nil {
		return err
	}

	// send funds
	_, _, err = vmctx.Runtime().Send(payer, types.SendMethodID, amt, nil)
	if err != nil {
		return errors.RevertErrorWrap(err, "could not send update funds")
	}

	return nil
}

// Separator is the separator used when concatenating channel and amount in a
// voucher signature.
const separator = 0x0

// SignVoucher creates the signature for the given combination of
// channel, amount, validAt (earliest block height for redeem) and from address.
// It does so by signing the following bytes: (channelID | 0x0 | amount | 0x0 | validAt)
func SignVoucher(channelID *types.ChannelID, amount types.AttoFIL, validAt *types.BlockHeight, addr address.Address, condition *types.Predicate, signer types.Signer) (types.Signature, error) {
	data, err := createVoucherSignatureData(channelID, amount, validAt, condition)
	if err != nil {
		return nil, err
	}
	return signer.SignBytes(data, addr)
}

// VerifyVoucherSignature returns whether the voucher's signature is valid
func VerifyVoucherSignature(payer address.Address, chid *types.ChannelID, amt types.AttoFIL, validAt *types.BlockHeight, condition *types.Predicate, sig []byte) bool {
	data, err := createVoucherSignatureData(chid, amt, validAt, condition)
	// the only error is failure to encode the values
	if err != nil {
		return false
	}
	return types.IsValidSignature(data, payer, sig)
}

func createVoucherSignatureData(channelID *types.ChannelID, amount types.AttoFIL, validAt *types.BlockHeight, condition *types.Predicate) ([]byte, error) {
	data := append(channelID.Bytes(), separator)
	data = append(data, amount.Bytes()...)
	data = append(data, separator)

	if condition != nil {
		data = append(data, condition.To.Bytes()...)

		encodedMethod, err := encoding.Encode(condition.Method)
		if err != nil {
			return data, err
		}
		data = append(data, encodedMethod...)

		encodedParams, err := abi.ToEncodedValues(condition.Params...)
		if err != nil {
			return []byte{}, err
		}
		data = append(data, encodedParams...)
	}
	return append(data, validAt.Bytes()...), nil
}

func withPayerChannels(ctx context.Context, st runtime.Storage, payer address.Address, f func(storage.Lookup) error) error {
	stateCid, err := actor.WithLookup(ctx, st, st.Head(), func(byPayer storage.Lookup) error {
		byChannelLookup, err := findByChannelLookup(ctx, st, byPayer, payer)
		if err != nil {
			return err
		}

		// run inner function
		err = f(byChannelLookup)
		if err != nil {
			return err
		}

		// commit channel lookup
		commitedCID, err := byChannelLookup.Commit(ctx)
		if err != nil {
			return err
		}

		// if all payers channels are gone, delete the payer
		if byChannelLookup.IsEmpty() {
			return byPayer.Delete(ctx, payer.String())
		}

		// set payers channels into primary lookup
		return byPayer.Set(ctx, payer.String(), commitedCID)
	})
	if err != nil {
		return err
	}

	return st.Commit(stateCid, st.Head())
}

func withPayerChannelsForReading(ctx context.Context, st runtime.Storage, payer address.Address, f func(storage.Lookup) error) error {
	return actor.WithLookupForReading(ctx, st, st.Head(), func(byPayer storage.Lookup) error {
		byChannelLookup, err := findByChannelLookup(ctx, st, byPayer, payer)
		if err != nil {
			return err
		}

		// run inner function
		return f(byChannelLookup)
	})
}

func findByChannelLookup(ctx context.Context, storage runtime.Storage, byPayer storage.Lookup, payer address.Address) (storage.Lookup, error) {
	var byChannelCID cid.Cid
	err := byPayer.Find(ctx, payer.String(), &byChannelCID)
	if err != nil {
		if err == hamt.ErrNotFound {
			return actor.LoadLookup(ctx, storage, cid.Undef)
		}
		return nil, err
	}

	return actor.LoadLookup(ctx, storage, byChannelCID)
}

// checkCondition combines params in the condition with the redeemerSuppliedParams, sends a message
// to the actor and method specified in the condition, and returns an error if one exists.
func checkCondition(vmctx invocationContext, channel *PaymentChannel) error {
	if channel.Condition == nil {
		return nil
	}

	_, _, err := vmctx.Runtime().Send(channel.Condition.To, channel.Condition.Method, types.ZeroAttoFIL, channel.Condition.Params)
	if err != nil {
		if errors.IsFault(err) {
			return err
		}
		return errors.NewCodedRevertErrorf(ErrConditionInvalid, "failed to validate voucher condition: %s", err)
	}
	return nil
}

// cacheCondition saves redeemer supplied conditions to the payment channel for
// future use
func cacheCondition(channel *PaymentChannel, condition *types.Predicate, redeemerSuppliedParams []interface{}) {
	if condition == nil {
		channel.Condition = nil
		return
	}

	// If new params have been provided or we don't yet have a cached condition,
	// cache the provided params and condition on the payment channel.
	if !channel.Redeemed || channel.Condition == nil || len(redeemerSuppliedParams) > 0 {
		newParams := condition.Params
		newParams = append(newParams, redeemerSuppliedParams...)

		newCachedCondition := *condition
		newCachedCondition.Params = newParams
		channel.Condition = &newCachedCondition
	}
}
