package consensus

import (
	"context"
	"math/big"

	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
	"github.com/filecoin-project/go-filecoin/internal/pkg/metrics"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

var errNegativeValueCt *metrics.Int64Counter
var errGasAboveBlockLimitCt *metrics.Int64Counter
var errInsufficientGasCt *metrics.Int64Counter
var errNonceTooLowCt *metrics.Int64Counter
var errNonceTooHighCt *metrics.Int64Counter

var (
	// These errors are only to be used by ApplyMessage; they shouldn't be
	// used in any other context as they are an implementation detail.
	errGasAboveBlockLimit = errors.NewRevertError("message gas limit above block gas limit")
	errGasPriceZero       = errors.NewRevertError("message gas price is zero")
	errNonceTooHigh       = errors.NewRevertError("nonce too high")
	errNonceTooLow        = errors.NewRevertError("nonce too low")
	errNonAccountActor    = errors.NewRevertError("message from non-account actor")
	errNegativeValue      = errors.NewRevertError("negative value")
	errInsufficientGas    = errors.NewRevertError("balance insufficient to cover transfer+gas")
	errInvalidSignature   = errors.NewRevertError("invalid signature by sender over message data")
	// TODO we'll eventually handle sending to self.
	errSelfSend = errors.NewRevertError("cannot send to self")
)

func init() {
	errNegativeValueCt = metrics.NewInt64Counter("consensus/msg_negative_value_err", "Number of negative valuedmessage")
	errGasAboveBlockLimitCt = metrics.NewInt64Counter("consensus/msg_gas_above_blk_limit_err", "Number of messages with gas above block limit")
	errInsufficientGasCt = metrics.NewInt64Counter("consensus/msg_insufficient_gas_err", "Number of messages with insufficient gas")
	errNonceTooLowCt = metrics.NewInt64Counter("consensus/msg_nonce_low_err", "Number of messages with nonce too low")
	errNonceTooHighCt = metrics.NewInt64Counter("consensus/msg_nonce_high_err", "Number of messages with nonce too high")
}

// DefaultMessageValidator validates incoming signed messages.
type DefaultMessageValidator struct {
	allowHighNonce bool
}

// NewDefaultMessageValidator creates a new default validator.
// A default validator checks for both permanent semantic problems (e.g. invalid signature)
// as well as temporary conditions which may change (e.g. actor can't cover gas limit).
func NewDefaultMessageValidator() *DefaultMessageValidator {
	return &DefaultMessageValidator{}
}

// NewOutboundMessageValidator creates a new default validator for outbound messages. This
// validator matches the default behaviour but allows nonces higher than the actor's current nonce
// (allowing multiple messages to enter the mpool at once).
func NewOutboundMessageValidator() *DefaultMessageValidator {
	return &DefaultMessageValidator{allowHighNonce: true}
}

// Validate checks that a message is semantically valid for processing, returning any
// invalidity as an error.
func (v *DefaultMessageValidator) Validate(ctx context.Context, msg *types.UnsignedMessage, fromActor *actor.Actor) error {
	if msg.From == msg.To {
		return errSelfSend
	}

	if msg.GasPrice.LessEqual(types.ZeroAttoFIL) {
		return errGasPriceZero
	}

	// Sender must be an account actor, or an empty actor which will be upgraded to an account actor
	// when the message is processed.
	if !(fromActor.Empty() || types.AccountActorCodeCid.Equals(fromActor.Code)) {
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

	if msg.CallSeqNum < fromActor.CallSeqNum {
		log.Debugf("Message: %s nonce lower than actor nonce: %s from actor: %s", msg.String(), fromActor.CallSeqNum, msg.From.String())
		errNonceTooLowCt.Inc(ctx, 1)
		return errNonceTooLow
	}

	if !v.allowHighNonce && msg.CallSeqNum > fromActor.CallSeqNum {
		log.Debugf("Message: %s nonce greater than actor nonce: %s from actor: %s", msg.String(), fromActor.CallSeqNum, msg.From.String())
		errNonceTooHighCt.Inc(ctx, 1)
		return errNonceTooHigh
	}

	return nil
}

// Check's whether the maximum gas charge + message value is within the actor's balance.
// Note that this is an imperfect test, since nested messages invoked by this one may transfer
// more value from the actor's balance.
func canCoverGasLimit(msg *types.UnsignedMessage, actor *actor.Actor) bool {
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
	validator *DefaultMessageValidator
}

// NewIngestionValidator creates a new validator with an api
func NewIngestionValidator(api ingestionValidatorAPI, cfg *config.MessagePoolConfig) *IngestionValidator {
	return &IngestionValidator{
		api:       api,
		cfg:       cfg,
		validator: &DefaultMessageValidator{allowHighNonce: true},
	}
}

// Validate validates the signed message.
// Errors probably mean the validation failed, but possibly indicate a failure to retrieve state
func (v *IngestionValidator) Validate(ctx context.Context, smsg *types.SignedMessage) error {
	// ensure message is properly signed
	if !smsg.VerifySignature() {
		return errInvalidSignature
	}

	// retrieve from actor
	msg := smsg.Message
	fromActor, err := v.api.GetActor(ctx, msg.From)
	if err != nil {
		if state.IsActorNotFoundError(err) {
			fromActor = &actor.Actor{}
		} else {
			return err
		}
	}

	// check that message nonce is not too high
	if msg.CallSeqNum > fromActor.CallSeqNum && msg.CallSeqNum-fromActor.CallSeqNum > v.cfg.MaxNonceGap {
		return errors.NewRevertErrorf("message nonce (%d) is too much greater than actor nonce (%d)", msg.CallSeqNum, fromActor.CallSeqNum)
	}

	return v.validator.Validate(ctx, &msg, fromActor)
}
