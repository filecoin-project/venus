package paymentbroker

import (
	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
	"github.com/filecoin-project/go-filecoin/vm/errors"
)

func init() {
	cbor.RegisterCborType(PaymentChannel{})
	cbor.RegisterCborType(Storage{})
	cbor.RegisterCborType(PaymentVoucher{})
}

// Signature signs an update request
type Signature = []byte

// allPaymentChannels are keyed by payer address
type allPaymentChannels map[string]accountPaymentChannels

// accountPaymentChannels are keyed by ChannelID
type accountPaymentChannels map[string]*PaymentChannel

// PaymentChannel records the intent to pay funds to a target account.
type PaymentChannel struct {
	Target         types.Address      `json:"target"`
	Amount         *types.TokenAmount `json:"amount"`
	AmountRedeemed *types.TokenAmount `json:"amount_redeemed"`
	Eol            *types.BlockHeight `json:"eol"`
}

// PaymentVoucher is a voucher for a payment channel that can be transferred off-chain but guarantees a future payment.
type PaymentVoucher struct {
	Channel   types.ChannelID   `json:"channel"`
	Payer     types.Address     `json:"payer"`
	Target    types.Address     `json:"target"`
	Amount    types.TokenAmount `json:"amount"`
	Signature Signature         `json:"signature"`
}

// Actor provides a mechanism for off chain payments.
// It allows the creation of payment Channels that hold funds for a target account
// and permits that account to withdraw funds only with a voucher signed by the
// channel's creator.
type Actor struct{}

// Storage is the payment broker's storage
type Storage struct {
	Channels allPaymentChannels
}

// NewStorage returns an empty Storage struct
func (pb *Actor) NewStorage() interface{} {
	return &Storage{}
}

// Exports returns the actor's exports
func (pb *Actor) Exports() exec.Exports {
	return paymentBrokerExports
}

var _ exec.ExecutableActor = (*Actor)(nil)

// NewPaymentBrokerActor returns a new payment broker actor.
func NewPaymentBrokerActor() (*types.Actor, error) {
	initStorage := &Storage{
		Channels: make(allPaymentChannels),
	}
	storageBytes, err := actor.MarshalStorage(initStorage)
	if err != nil {
		return nil, err
	}
	return types.NewActorWithMemory(types.PaymentBrokerActorCodeCid, types.NewTokenAmount(0), storageBytes), nil
}

var paymentBrokerExports = exec.Exports{
	"close": &exec.FunctionSignature{
		Params: []abi.Type{abi.Address, abi.ChannelID, abi.TokenAmount, abi.Bytes},
		Return: nil,
	},
	"createChannel": &exec.FunctionSignature{
		Params: []abi.Type{abi.Address, abi.BlockHeight},
		Return: []abi.Type{abi.ChannelID},
	},
	"extend": &exec.FunctionSignature{
		Params: []abi.Type{abi.ChannelID, abi.BlockHeight},
		Return: nil,
	},
	"ls": &exec.FunctionSignature{
		Params: []abi.Type{abi.Address},
		Return: []abi.Type{abi.Bytes},
	},
	"reclaim": &exec.FunctionSignature{
		Params: []abi.Type{abi.ChannelID},
		Return: nil,
	},
	"update": &exec.FunctionSignature{
		Params: []abi.Type{abi.Address, abi.ChannelID, abi.TokenAmount, abi.Bytes},
		Return: nil,
	},
	"voucher": &exec.FunctionSignature{
		Params: []abi.Type{abi.ChannelID, abi.TokenAmount},
		Return: []abi.Type{abi.Bytes},
	},
}

// CreateChannel creates a new payment channel from the caller to the target.
// The value attached to the invocation is used as the deposit, and the channel
// will expire and return all of its money to the owner after the given block height.
func (pb *Actor) CreateChannel(ctx *vm.Context, target types.Address, eol *types.BlockHeight) (*types.ChannelID, uint8, error) {
	var storage Storage
	ret, err := actor.WithStorage(ctx, &storage, func() (interface{}, error) {
		// require that from account be an account actor to ensure nonce is a valid id
		if !ctx.IsFromAccountActor() {
			return nil, errors.NewRevertError("Only account actors may create payment channels")
		}

		byPayer, found := storage.Channels[ctx.Message().From.String()]
		if !found {
			byPayer = make(map[string]*PaymentChannel)
			storage.Channels[ctx.Message().From.String()] = byPayer
		}

		channelID := types.NewChannelID(ctx.Message().Nonce)

		if _, found := byPayer[channelID.String()]; found {
			return nil, errors.NewRevertError("Duplicate create channel attempt")
		}

		paymentChannel := &PaymentChannel{
			Target:         target,
			Amount:         ctx.Message().Value,
			AmountRedeemed: types.NewTokenAmount(0),
			Eol:            eol,
		}

		byPayer[channelID.String()] = paymentChannel

		return channelID, nil
	})
	if err != nil {
		return nil, 1, err
	}

	return ret.(*types.ChannelID), 0, nil
}

// Update is called by the target account to withdraw funds with authorization from the payer.
// This method is exactly like Close except it doesn't close the channel.
// This is useful when you want to checkpoint the value in a payment, but continue to use the
// channel afterwards. The amt represents the total funds authorized so far, so that subsequent
// calls to Update will only transfer the difference between the given amt and the greatest
// amt taken so far. A series of channel transactions might look like this:
//                                Payer: 2000, Target: 0, Channel: 0
// payer createChannel(1000)   -> Payer: 1000, Target: 0, Channel: 1000
// target Update(100)          -> Payer: 1000, Target: 100, Channel: 900
// target Update(200)          -> Payer: 1000, Target: 200, Channel: 800
// target Close(500)           -> Payer: 1500, Target: 500, Channel: 0
//
func (pb *Actor) Update(ctx *vm.Context, payer types.Address, chid *types.ChannelID, amt *types.TokenAmount, sig Signature) (uint8, error) {
	var storage Storage
	_, err := actor.WithStorage(ctx, &storage, func() (interface{}, error) {

		// TODO: check the signature against the other voucher components.

		channel, err := findChannel(&storage, payer, chid)
		if err != nil {
			return nil, err
		}

		err = updateChannel(ctx, ctx.Message().From, channel, amt)
		return nil, err
	})
	if err != nil {
		return 1, err
	}

	return 0, nil
}

// Close first executes the logic performed in the the Update method, then returns all
// funds remaining in the channel to the payer account and deletes the channel.
func (pb *Actor) Close(ctx *vm.Context, payer types.Address, chid *types.ChannelID, amt *types.TokenAmount, sig Signature) (uint8, error) {
	var storage Storage
	_, err := actor.WithStorage(ctx, &storage, func() (interface{}, error) {

		// TODO: check the signature against the other voucher components.

		channel, err := findChannel(&storage, payer, chid)
		if err != nil {
			return nil, err
		}

		err = updateChannel(ctx, ctx.Message().From, channel, amt)
		if err != nil {
			return nil, err
		}

		// return funds to payer
		err = reclaim(ctx, &storage, payer, chid, channel)
		return nil, err
	})
	if err != nil {
		return 1, err
	}

	return 0, nil
}

// Extend can be used by the owner of a channel to add more funds to it and
// extend the Channels lifespan.
func (pb *Actor) Extend(ctx *vm.Context, chid *types.ChannelID, eol *types.BlockHeight) (uint8, error) {
	var storage Storage
	_, err := actor.WithStorage(ctx, &storage, func() (interface{}, error) {
		channel, err := findChannel(&storage, ctx.Message().From, chid)
		if err != nil {
			return nil, err
		}

		// eol can only be increased
		if channel.Eol.GreaterThan(eol) {
			return nil, errors.NewRevertErrorf("payment channel eol may not be decreased (%s < %s)", eol, channel.Eol)
		}

		// set new eol
		channel.Eol = eol

		// increment the value
		channel.Amount = channel.Amount.Add(ctx.Message().Value)

		// return funds to payer
		return nil, err
	})
	if err != nil {
		return 1, err
	}

	return 0, nil
}

// Reclaim is used by the owner of a channel to reclaim unspent funds in timed
// out payment Channels they own.
func (pb *Actor) Reclaim(ctx *vm.Context, chid *types.ChannelID) (uint8, error) {
	var storage Storage
	_, err := actor.WithStorage(ctx, &storage, func() (interface{}, error) {
		channel, err := findChannel(&storage, ctx.Message().From, chid)
		if err != nil {
			return nil, err
		}

		// reclaim may only be called at or after Eol
		if ctx.BlockHeight().LessThan(channel.Eol) {
			return nil, errors.NewRevertErrorf("payment channel may not reclaimed before eol (%s)", channel.Eol)
		}

		// return funds to payer
		err = reclaim(ctx, &storage, ctx.Message().From, chid, channel)
		return nil, err
	})
	if err != nil {
		return 1, err
	}

	return 0, nil
}

// Voucher takes a channel id and amount creates a new unsigned PaymentVoucher against the given channel.
// It errors if the channel doesn't exist or contains less than request amount.
func (pb *Actor) Voucher(ctx *vm.Context, chid *types.ChannelID, amount *types.TokenAmount) ([]byte, uint8, error) {
	var storage Storage
	ret, err := actor.WithStorage(ctx, &storage, func() (interface{}, error) {
		channel, err := findChannel(&storage, ctx.Message().From, chid)
		if err != nil {
			return nil, err
		}

		// voucher must be for less than total amount in channel
		if channel.Amount.LessThan(amount) {
			return nil, errors.NewRevertErrorf("voucher amount exceeds amount in channel (%s > %s)", amount, channel.Amount)
		}

		// return voucher
		voucher := PaymentVoucher{
			Channel: *chid,
			Payer:   ctx.Message().From,
			Target:  channel.Target,
			Amount:  *amount,
		}

		return cbor.DumpObject(voucher)
	})
	if err != nil {
		return nil, 1, err
	}

	return ret.([]byte), 0, nil
}

// Ls returns all payment channels for a given payer address.
// The slice of channels will be returned as cbor encoded map from string channelId to PaymentChannel.
func (pb *Actor) Ls(ctx *vm.Context, payer types.Address) ([]byte, uint8, error) {
	var storage Storage
	ret, err := actor.WithStorage(ctx, &storage, func() (interface{}, error) {
		byPayer, found := storage.Channels[payer.String()]
		if !found {
			byPayer = make(map[string]*PaymentChannel)
		}

		return cbor.DumpObject(byPayer)
	})
	if err != nil {
		return nil, 1, err
	}

	return ret.([]byte), 0, nil
}

func findChannel(storage *Storage, payer types.Address, chid *types.ChannelID) (*PaymentChannel, error) {
	actorsChannels, found := storage.Channels[payer.String()]
	if !found {
		return nil, errors.NewRevertErrorf("payment channel %s for %s is unknown", chid, payer)
	}

	channel, found := actorsChannels[chid.String()]
	if !found {
		return nil, errors.NewRevertErrorf("payment channel %s for %s is unknown", chid, payer)
	}

	return channel, nil
}

func updateChannel(ctx *vm.Context, target types.Address, channel *PaymentChannel, amt *types.TokenAmount) error {
	if target != channel.Target {
		return errors.NewRevertErrorf("attempt to redeem channel from wrong target account")
	}

	if ctx.BlockHeight().GreaterEqual(channel.Eol) {
		return errors.NewRevertErrorf("block height has exceeded channel's end of life")
	}

	if amt.GreaterThan(channel.Amount) {
		return errors.NewRevertErrorf("update amount %s exceeds funds in channel", amt)
	}

	if amt.LessEqual(channel.AmountRedeemed) {
		return errors.NewRevertErrorf("update amount %s has already been redeemed", amt)
	}

	// transfer funds to sender
	updateAmount := amt.Sub(channel.AmountRedeemed)
	_, _, err := ctx.Send(ctx.Message().From, "", updateAmount, nil)
	if err != nil {
		return err
	}

	// update amount redeemed from this channel
	channel.AmountRedeemed = amt

	return nil
}

func reclaim(ctx *vm.Context, storage *Storage, payer types.Address, chid *types.ChannelID, channel *PaymentChannel) error {
	amt := channel.Amount.Sub(channel.AmountRedeemed)
	if amt.LessEqual(types.ZeroToken) {
		return nil
	}

	// clean up
	actorsChannels, found := storage.Channels[payer.String()]
	if !found {
		return errors.NewRevertError("unexpected error closing channel")
	}

	delete(actorsChannels, chid.String())
	if len(actorsChannels) == 0 {
		delete(storage.Channels, payer.String())
	}

	// send funds
	_, _, err := ctx.Send(payer, "", amt, nil)
	if err != nil {
		return errors.RevertErrorWrap(err, "could not send update funds")
	}

	return nil
}
