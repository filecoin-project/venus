package consensus

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	builtin0 "github.com/filecoin-project/specs-actors/actors/builtin"
	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/constants"
	"github.com/filecoin-project/venus/internal/pkg/crypto"
	"github.com/filecoin-project/venus/internal/pkg/metrics"
	"github.com/filecoin-project/venus/internal/pkg/state"
	"github.com/filecoin-project/venus/internal/pkg/types"
)

var dropNonAccountCt *metrics.Int64Counter
var dropInsufficientGasCt *metrics.Int64Counter
var dropNonceTooLowCt *metrics.Int64Counter
var dropNonceTooHighCt *metrics.Int64Counter

var invReceiverUndefCt *metrics.Int64Counter
var invSenderUndefCt *metrics.Int64Counter
var invValueAboveMaxCt *metrics.Int64Counter
var invParamsNilCt *metrics.Int64Counter
var invGasPriceNegativeCt *metrics.Int64Counter
var invGasBelowMinimumCt *metrics.Int64Counter
var invNegativeValueCt *metrics.Int64Counter
var invGasAboveBlockLimitCt *metrics.Int64Counter

// The maximum allowed message value.
var msgMaxValue = types.NewAttoFILFromFIL(2e9)

// These gas cost values must match those in vm/gas.
// TODO: Look up gas costs from the same place the VM gets them, keyed by epoch. https://github.com/filecoin-project/venus/issues/3955
const onChainMessageBase = types.Unit(0)
const onChainMessagePerByte = types.Unit(2)

func init() {
	dropNonAccountCt = metrics.NewInt64Counter("consensus/msg_non_account_sender", "Count of dropped messages with non-account sender")
	dropInsufficientGasCt = metrics.NewInt64Counter("consensus/msg_insufficient_gas_err", "Count of dropped messages with insufficient gas")
	dropNonceTooLowCt = metrics.NewInt64Counter("consensus/msg_nonce_low_err", "Count of dropped  messages with nonce too low")
	dropNonceTooHighCt = metrics.NewInt64Counter("consensus/msg_nonce_high_err", "Count of dropped  messages with nonce too high")

	invReceiverUndefCt = metrics.NewInt64Counter("consensus/msg_undef_receiver", "Count of")
	invSenderUndefCt = metrics.NewInt64Counter("consensus/msg_undef_sender", "Count of")
	invValueAboveMaxCt = metrics.NewInt64Counter("consensus/msg_value_max", "Count of")
	invParamsNilCt = metrics.NewInt64Counter("consensus/msg_params_nil", "Count of")
	invGasPriceNegativeCt = metrics.NewInt64Counter("consensus/msg_gasprice_negative", "Count of")
	invGasBelowMinimumCt = metrics.NewInt64Counter("consensus/msg_gaslimit_min", "Count of")
	invNegativeValueCt = metrics.NewInt64Counter("consensus/msg_value_negative", "Count of invalid negative messages with negative value")
	invGasAboveBlockLimitCt = metrics.NewInt64Counter("consensus/msg_gaslimit_max", "Count of invalid messages with gas above block limit")
}

// MessageSelectionChecker checks for miner penalties on signed messages
type MessagePenaltyChecker struct {
	api penaltyCheckerAPI
}

// penaltyCheckerAPI allows the validator to access latest state
type penaltyCheckerAPI interface {
	Head() block.TipSetKey
	GetActorAt(ctx context.Context, tipKey block.TipSetKey, addr address.Address) (*types.Actor, error)
}

func NewMessagePenaltyChecker(api penaltyCheckerAPI) *MessagePenaltyChecker {
	return &MessagePenaltyChecker{
		api: api,
	}
}

// PenaltyCheck checks that a message is semantically valid for processing without
// causing miner penality.  It treats any miner penalty condition as an error.
func (v *MessagePenaltyChecker) PenaltyCheck(ctx context.Context, msg *types.UnsignedMessage) error {
	fromActor, err := v.api.GetActorAt(ctx, v.api.Head(), msg.From)
	if err != nil {
		return err
	}
	// Sender should not be an empty actor
	if fromActor == nil || fromActor.Empty() {
		return fmt.Errorf("sender %s is missing/empty: %s", msg.From, msg)
	}

	// Sender must be an account actor.
	if !(builtin0.AccountActorCodeID.Equals(fromActor.Code.Cid)) && !(builtin2.AccountActorCodeID.Equals(fromActor.Code.Cid)) {
		dropNonAccountCt.Inc(ctx, 1)
		return fmt.Errorf("sender %s is non-account actor with code %s: %s", msg.From, fromActor.Code, msg)
	}

	// Avoid processing messages for actors that cannot pay.
	if !canCoverGasLimit(msg, fromActor) {
		dropInsufficientGasCt.Inc(ctx, 1)
		return fmt.Errorf("insufficient funds from sender %s to cover value and gas cost: %s ", msg.From, msg)
	}

	if msg.CallSeqNum < fromActor.CallSeqNum {
		dropNonceTooLowCt.Inc(ctx, 1)
		return fmt.Errorf("nonce %d lower than expected %d: %s", msg.CallSeqNum, fromActor.CallSeqNum, msg)
	}

	if msg.CallSeqNum > fromActor.CallSeqNum {
		dropNonceTooHighCt.Inc(ctx, 1)
		return fmt.Errorf("nonce %d greater than expected: %d: %s", msg.CallSeqNum, fromActor.CallSeqNum, msg)
	}

	return nil
}

// Check's whether the maximum gas charge + message value is within the actor's balance.
// Note that this is an imperfect test, since nested messages invoked by this one may transfer
// more value from the actor's balance.
func canCoverGasLimit(msg *types.UnsignedMessage, actor *types.Actor) bool {
	// balance >= (gasprice*gasLimit + value)
	gascost := big.Mul(abi.NewTokenAmount(msg.GasFeeCap.Int.Int64()), abi.NewTokenAmount(int64(msg.GasLimit)))
	expense := big.Add(gascost, abi.NewTokenAmount(msg.Value.Int.Int64()))
	return actor.Balance.GreaterThanEqual(expense)
}

// DefaultMessageSyntaxValidator checks basic conditions independent of current state
type DefaultMessageSyntaxValidator struct{}

func NewMessageSyntaxValidator() *DefaultMessageSyntaxValidator {
	return &DefaultMessageSyntaxValidator{}
}

// ValidateSignedMessageSyntax validates signed message syntax and state-independent invariants.
// Used for incoming messages over pubsub and secp messages included in blocks.
func (v *DefaultMessageSyntaxValidator) ValidateSignedMessageSyntax(ctx context.Context, smsg *types.SignedMessage) error {
	msg := &smsg.Message
	var msgLen int
	if smsg.Signature.Type == crypto.SigTypeBLS {
		enc, err := smsg.Message.Marshal()
		if err != nil {
			return errors.Wrapf(err, "failed to calculate message size")
		}
		msgLen = len(enc)
	} else {
		enc, err := smsg.Marshal()
		if err != nil {
			return errors.Wrapf(err, "failed to calculate message size")
		}
		msgLen = len(enc)
	}
	return v.validateMessageSyntaxShared(ctx, msg, msgLen)
}

// ValidateUnsignedMessageSyntax validates unisigned message syntax and state-independent invariants.
// Used for bls messages included in blocks.
func (v *DefaultMessageSyntaxValidator) ValidateUnsignedMessageSyntax(ctx context.Context, msg *types.UnsignedMessage) error {
	enc, err := msg.Marshal()
	if err != nil {
		return errors.Wrapf(err, "failed to calculate message size")
	}
	msgLen := len(enc)
	return v.validateMessageSyntaxShared(ctx, msg, msgLen)
}

func (v *DefaultMessageSyntaxValidator) validateMessageSyntaxShared(ctx context.Context, msg *types.UnsignedMessage, msgLen int) error {
	if msg.Version != types.MessageVersion {
		return fmt.Errorf("version %d, expected %d", msg.Version, types.MessageVersion)
	}

	if msg.To.Empty() {
		invReceiverUndefCt.Inc(ctx, 1)
		return fmt.Errorf("empty receiver: %s", msg)
	}
	if msg.From.Empty() {
		invSenderUndefCt.Inc(ctx, 1)
		return fmt.Errorf("empty sender: %s", msg)
	}
	// The spec calls for validating a non-negative call sequence num, but by
	// the time it's decoded into a uint64 the check is already passed

	if msg.Value.LessThan(big.Zero()) {
		invNegativeValueCt.Inc(ctx, 1)
		return fmt.Errorf("negative value %s: %s", msg.Value, msg)
	}
	if msg.Value.GreaterThan(msgMaxValue) {
		invValueAboveMaxCt.Inc(ctx, 1)
		return fmt.Errorf("value %s exceeds max %s: %s", msg.Value, msgMaxValue, msg)
	}
	// The spec calls for validating a non-negative method num, but by the
	// time it's decoded into a uint64 the check is already passed

	if msg.Params == nil {
		invParamsNilCt.Inc(ctx, 1)
		return fmt.Errorf("nil params (should be empty-array): %s", msg)
	}
	if msg.GasFeeCap.LessThan(types.ZeroAttoFIL) {
		invGasPriceNegativeCt.Inc(ctx, 1)
		return fmt.Errorf("negative gas price %s: %s", msg.GasFeeCap, msg)
	}
	// The minimum gas limit ensures the sender has enough balance to pay for inclusion of the message in the chain
	// *at all*. Without this, a message could hit out-of-gas but the sender pay nothing.
	// NOTE(anorth): this check has been moved to execution time, and the miner is penalized for including
	// such a message. We can probably remove this.
	minMsgGas := onChainMessageBase + onChainMessagePerByte*types.Unit(msgLen)
	if msg.GasLimit < minMsgGas {
		invGasBelowMinimumCt.Inc(ctx, 1)
		return fmt.Errorf("gas limit %d below minimum %d to cover message size: %s", msg.GasLimit, minMsgGas, msg)
	}
	if msg.GasLimit > constants.BlockGasLimit {
		invGasAboveBlockLimitCt.Inc(ctx, 1)
		return fmt.Errorf("gas limit %d exceeds block limit %d: %s", msg.GasLimit, constants.BlockGasLimit, msg)
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
	GetTipSet(key block.TipSetKey) (*block.TipSet, error)
	AccountStateView(baseKey block.TipSetKey, height abi.ChainEpoch) (state.AccountStateView, error)
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
	headTipset, err := v.api.GetTipSet(head)
	if err != nil {
		return errors.Wrapf(err, "failed to get height: %v", err)
	}

	view, err := v.api.AccountStateView(head, headTipset.At(0).Height)
	if err != nil {
		return errors.Wrapf(err, "failed to load state at %v", head)
	}

	sigValidator := state.NewSignatureValidator(view)

	// ensure message is properly signed
	if err := sigValidator.ValidateMessageSignature(ctx, smsg); err != nil {
		return errors.Wrap(err, fmt.Errorf("invalid signature by sender over message data").Error())
	}
	return nil
}
