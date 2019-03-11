package consensus

import (
	"context"
	"math/big"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/types"
)

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

	// Sender must be an account actor, or an empty actor which will be upgraded to an account actor
	// when the message is processed.
	if !(fromActor.Empty() || account.IsAccount(fromActor)) {
		return errNonAccountActor
	}

	if msg.Value.IsNegative() {
		log.Info("Cannot transfer negative value", fromActor, msg.Value)
		return errNegativeValue
	}

	if msg.GasLimit > types.BlockGasLimit {
		log.Info("Message gas limit above block limit", fromActor, msg, types.BlockGasLimit)
		return errGasAboveBlockLimit
	}

	// Avoid processing messages for actors that cannot pay.
	if !canCoverGasLimit(msg, fromActor) {
		log.Info("Insufficient funds to cover gas limit: ", fromActor, msg)
		return errInsufficientGas
	}

	if msg.Nonce < fromActor.Nonce {
		log.Info("Nonce too low: ", msg.Nonce, fromActor.Nonce, fromActor, msg)
		return errNonceTooLow
	}

	if !v.allowHighNonce && msg.Nonce > fromActor.Nonce {
		log.Info("Nonce too high: ", msg.Nonce, fromActor.Nonce, fromActor, msg)
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
