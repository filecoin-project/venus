package consensus

import (
	"context"
	"math/big"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/metrics"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm/errors"
)

var errNegativeValueCt *metrics.Int64Counter
var errGasAboveBlockLimitCt *metrics.Int64Counter
var errInsufficientGasCt *metrics.Int64Counter
var errNonceTooLowCt *metrics.Int64Counter
var errNonceTooHighCt *metrics.Int64Counter

func init() {
	errNegativeValueCt = metrics.NewInt64Counter("consensus/msg_negative_value_err", "Number of negative valuedmessage")
	errGasAboveBlockLimitCt = metrics.NewInt64Counter("consensus/msg_gas_above_blk_limit_err", "Number of messages with gas above block limit")
	errInsufficientGasCt = metrics.NewInt64Counter("consensus/msg_insufficient_gas_err", "Number of messages with insufficient gas")
	errNonceTooLowCt = metrics.NewInt64Counter("consensus/msg_nonce_low_err", "Number of messages with nonce too low")
	errNonceTooHighCt = metrics.NewInt64Counter("consensus/msg_nonce_high_err", "Number of messages with nonce too high")
}

// SignedMessageValidator validates incoming signed messages.
type SignedMessageValidator interface {
	// Validate checks that a message is semantically valid for processing, returning any
	// invalidity as an error
	Validate(ctx context.Context, msg *types.SignedMessage, fromActor *actor.Actor) error
}

type defaultMessageValidator struct {
	allowHighNonce bool
}

// NewDefaultMessageValidator creates a new default validator.
// A default validator checks for both permanent semantic problems (e.g. invalid signature)
// as well as temporary conditions which may change (e.g. actor can't cover gas limit).
func NewDefaultMessageValidator() SignedMessageValidator {
	return &defaultMessageValidator{}
}

// NewOutboundMessageValidator creates a new default validator for outbound messages. This
// validator matches the default behaviour but allows nonces higher than the actor's current nonce
// (allowing multiple messages to enter the mpool at once).
func NewOutboundMessageValidator() SignedMessageValidator {
	return &defaultMessageValidator{allowHighNonce: true}
}

var _ SignedMessageValidator = (*defaultMessageValidator)(nil)

func (v *defaultMessageValidator) Validate(ctx context.Context, msg *types.SignedMessage, fromActor *actor.Actor) error {
	if !msg.VerifySignature() {
		return errInvalidSignature
	}

	if msg.From == msg.To {
		return errSelfSend
	}

	if msg.GasPrice.LessEqual(types.ZeroAttoFIL) {
		return errGasPriceZero
	}

	// Sender must be an account actor, or an empty actor which will be upgraded to an account actor
	// when the message is processed.
	if !(fromActor.Empty() || account.IsAccount(fromActor)) {
		return errNonAccountActor
	}

	if msg.Value.IsNegative() {
		log.Debugf("Cannot transfer negative value: %s from actor: %s", msg.Value.String(), msg.From.String())
		errNegativeValueCt.Inc(ctx, 1)
		return errNegativeValue
	}

	if msg.GasLimit > types.BlockGasLimit {
		log.Debugf("Message: %s gas limit from actor: %s above block limit: %s", msg.String(), msg.From.String(), string(types.BlockGasLimit))
		errGasAboveBlockLimitCt.Inc(ctx, 1)
		return errGasAboveBlockLimit
	}

	// Avoid processing messages for actors that cannot pay.
	if !canCoverGasLimit(msg, fromActor) {
		log.Debugf("Insufficient funds for message: %s to cover gas limit from actor: %s", msg.String(), msg.From.String())
		errInsufficientGasCt.Inc(ctx, 1)
		return errInsufficientGas
	}

	if msg.Nonce < fromActor.Nonce {
		log.Debugf("Message: %s nonce lower than actor nonce: %s from actor: %s", msg.String(), fromActor.Nonce, msg.From.String())
		errNonceTooLowCt.Inc(ctx, 1)
		return errNonceTooLow
	}

	if !v.allowHighNonce && msg.Nonce > fromActor.Nonce {
		log.Debugf("Message: %s nonce greater than actor nonce: %s from actor: %s", msg.String(), fromActor.Nonce, msg.From.String())
		errNonceTooHighCt.Inc(ctx, 1)
		return errNonceTooHigh
	}

	return nil
}

// Check's whether the maximum gas charge + message value is within the actor's balance.
// Note that this is an imperfect test, since nested messages invoked by this one may transfer
// more value from the actor's balance.
func canCoverGasLimit(msg *types.SignedMessage, actor *actor.Actor) bool {
	maximumGasCharge := msg.GasPrice.MulBigInt(big.NewInt(int64(msg.GasLimit)))
	return maximumGasCharge.LessEqual(actor.Balance.Sub(msg.Value))
}

// IngestionValidatorAPI allows the validator to access latest state
type ingestionValidatorAPI interface {
	GetActor(context.Context, address.Address) (*actor.Actor, error)
}

// IngestionValidator can access latest state and runs additional checks to mitigate DoS attacks
type IngestionValidator struct {
	api       ingestionValidatorAPI
	cfg       *config.MessagePoolConfig
	validator defaultMessageValidator
}

// NewIngestionValidator creates a new validator with an api
func NewIngestionValidator(api ingestionValidatorAPI, cfg *config.MessagePoolConfig) *IngestionValidator {
	return &IngestionValidator{
		api:       api,
		cfg:       cfg,
		validator: defaultMessageValidator{allowHighNonce: true},
	}
}

// Validate validates the signed message.
// Errors probably mean the validation failed, but possibly indicate a failure to retrieve state
func (v *IngestionValidator) Validate(ctx context.Context, msg *types.SignedMessage) error {
	// retrieve from actor
	fromActor, err := v.api.GetActor(ctx, msg.From)
	if err != nil {
		if state.IsActorNotFoundError(err) {
			fromActor = &actor.Actor{}
		} else {
			return err
		}
	}

	// check that message nonce is not too high
	if msg.Nonce > fromActor.Nonce && msg.Nonce-fromActor.Nonce > v.cfg.MaxNonceGap {
		return errors.NewRevertErrorf("message nonce (%d) is too much greater than actor nonce (%d)", msg.Nonce, fromActor.Nonce)
	}

	return v.validator.Validate(ctx, msg, fromActor)
}
