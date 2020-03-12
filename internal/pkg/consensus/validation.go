package consensus

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	specsbig "github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/metrics"
	"github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
)

var errNegativeValueCt *metrics.Int64Counter
var errGasAboveBlockLimitCt *metrics.Int64Counter
var errInsufficientGasCt *metrics.Int64Counter
var errNonceTooLowCt *metrics.Int64Counter
var errNonceTooHighCt *metrics.Int64Counter

var (
	// These errors are only to be used by ApplyMessage; they shouldn't be
	// used in any other context as they are an implementation detail.
	errGasAboveBlockLimit = fmt.Errorf("message gas limit above block gas limit")
	errGasPriceZero       = fmt.Errorf("message gas price is zero")
	errNonceTooHigh       = fmt.Errorf("nonce too high")
	errNonceTooLow        = fmt.Errorf("nonce too low")
	errNonAccountActor    = fmt.Errorf("message from non-account actor")
	errNegativeValue      = fmt.Errorf("negative value")
	errInsufficientGas    = fmt.Errorf("balance insufficient to cover transfer+gas")
	errInvalidSignature   = fmt.Errorf("invalid signature by sender over message data")
	errEmptySender        = fmt.Errorf("message sends from empty actor")
)

func init() {
	errNegativeValueCt = metrics.NewInt64Counter("consensus/msg_negative_value_err", "Number of negative valuedmessage")
	errGasAboveBlockLimitCt = metrics.NewInt64Counter("consensus/msg_gas_above_blk_limit_err", "Number of messages with gas above block limit")
	errInsufficientGasCt = metrics.NewInt64Counter("consensus/msg_insufficient_gas_err", "Number of messages with insufficient gas")
	errNonceTooLowCt = metrics.NewInt64Counter("consensus/msg_nonce_low_err", "Number of messages with nonce too low")
	errNonceTooHighCt = metrics.NewInt64Counter("consensus/msg_nonce_high_err", "Number of messages with nonce too high")
}

// MessageSelectionChecker checks for miner penalties on signed messages
type MessagePenaltyChecker struct {
	api penaltyCheckerAPI
}

// penaltyCheckerAPI allows the validator to access latest state
type penaltyCheckerAPI interface {
	Head() block.TipSetKey
	GetActorAt(ctx context.Context, tipKey block.TipSetKey, addr address.Address) (*actor.Actor, error)
}

func NewMessagePenaltyChecker(api penaltyCheckerAPI) *MessagePenaltyChecker {
	return &MessagePenaltyChecker{
		api: api,
	}
}

// PenaltyCheck checks that a message is semantically valid for processing without
// causing miner penality.  It treats any miner penalty condtion as an error.
func (v *MessagePenaltyChecker) PenaltyCheck(ctx context.Context, msg *types.UnsignedMessage) error {
	fromActor, err := v.api.GetActorAt(ctx, v.api.Head(), msg.From)
	if err != nil {
		return err
	}
	// Sender should not be an empty actor
	if fromActor == nil || fromActor.Empty() {
		return errEmptySender
	}

	// Sender must be an account actor.
	if !(builtin.AccountActorCodeID.Equals(fromActor.Code.Cid)) {
		return errNonAccountActor
	}

	// Avoid processing messages for actors that cannot pay.
	if !canCoverGasLimit(msg, fromActor) {
		log.Debugf("Insufficient funds for message: %s to cover gas limit from actor: %s", msg, msg.From)
		errInsufficientGasCt.Inc(ctx, 1)
		return errInsufficientGas
	}

	if msg.CallSeqNum < fromActor.CallSeqNum {
		log.Debugf("Message: %s nonce lower than actor nonce: %s from actor: %s", msg, fromActor.CallSeqNum, msg.From)
		errNonceTooLowCt.Inc(ctx, 1)
		return errNonceTooLow
	}

	if msg.CallSeqNum > fromActor.CallSeqNum {
		log.Debugf("Message: %s nonce greater than actor nonce: %s from actor: %s", msg, fromActor.CallSeqNum, msg.From)
		errNonceTooHighCt.Inc(ctx, 1)
		return errNonceTooHigh
	}

	return nil
}

// Check's whether the maximum gas charge + message value is within the actor's balance.
// Note that this is an imperfect test, since nested messages invoked by this one may transfer
// more value from the actor's balance.
func canCoverGasLimit(msg *types.UnsignedMessage, actor *actor.Actor) bool {
	// balance >= (gasprice*gasLimit + value)
	gascost := specsbig.Mul(abi.NewTokenAmount(msg.GasPrice.Int.Int64()), abi.NewTokenAmount(int64(msg.GasLimit)))
	expense := specsbig.Add(gascost, abi.NewTokenAmount(msg.Value.Int.Int64()))
	return actor.Balance.GreaterThanEqual(expense)
}

// MessageSyntaxValidator checks basic conditions independent of current state
type MessageSyntaxValidator struct{}

func NewMessageSyntaxValidator() *MessageSyntaxValidator {
	return &MessageSyntaxValidator{}
}

func (v *MessageSyntaxValidator) Validate(ctx context.Context, smsg *types.SignedMessage) error {
	// check non-state dependent invariants
	msg := smsg.Message
	if msg.GasLimit > types.BlockGasLimit {
		log.Debugf("Message: %s gas limit from actor: %s above block limit: %s", msg, msg.From, types.BlockGasLimit)
		errGasAboveBlockLimitCt.Inc(ctx, 1)
		return errGasAboveBlockLimit
	}

	if msg.Value.LessThan(specsbig.Zero()) {
		log.Debugf("Cannot transfer negative value: %s from actor: %s", msg.Value, msg.From)
		errNegativeValueCt.Inc(ctx, 1)
		return errNegativeValue
	}

	if msg.GasPrice.LessThanEqual(types.ZeroAttoFIL) {
		return errGasPriceZero
	}

	return nil
}

// MessageSignatureValidator validates message signatures
type MessageSignatureValidator struct {
	api signatureValidatorAPI
}

// signatureValidatorAPI allows the validator to access state needed for signature checking
type signatureValidatorAPI interface {
	Head() block.TipSetKey
	AccountStateView(baseKey block.TipSetKey) (state.AccountStateView, error)
}

func NewMessageSignatureValidator(api signatureValidatorAPI) *MessageSignatureValidator {
	return &MessageSignatureValidator{
		api: api,
	}
}

// Validate validates the signed message signature. Errors probably mean the
//  validation failed, but possibly indicate a failure to retrieve state.
func (v *MessageSignatureValidator) Validate(ctx context.Context, smsg *types.SignedMessage) error {
	head := v.api.Head()
	view, err := v.api.AccountStateView(head)
	if err != nil {
		return errors.Wrapf(err, "failed to load state at %v", head)
	}

	sigValidator := state.NewSignatureValidator(view)

	// ensure message is properly signed
	if err := sigValidator.ValidateMessageSignature(ctx, smsg); err != nil {
		return errors.Wrap(err, errInvalidSignature.Error())
	}
	return nil
}
