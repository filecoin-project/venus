package consensus

import (
	"bytes"
	"context"
	"fmt"

	"github.com/filecoin-project/go-state-types/big"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/metrics"
	"github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/types"
)

var dropNonAccountCt *metrics.Int64Counter
var dropInsufficientGasCt *metrics.Int64Counter
var dropNonceTooLowCt *metrics.Int64Counter
var dropNonceTooHighCt *metrics.Int64Counter

var invReceiverUndefCt *metrics.Int64Counter
var invSenderUndefCt *metrics.Int64Counter
var invValueAboveMaxCt *metrics.Int64Counter
var invParamsNilCt *metrics.Int64Counter // nolint
var invGasPriceNegativeCt *metrics.Int64Counter
var invGasBelowMinimumCt *metrics.Int64Counter
var invNegativeValueCt *metrics.Int64Counter
var invGasAboveBlockLimitCt *metrics.Int64Counter

// The maximum allowed message value.
var msgMaxValue = types.NewAttoFILFromFIL(2e9)

// These gas cost values must match those in vm/gas.
// TODO: Look up gas costs from the same place the VM gets them, keyed by epoch. https://github.com/filecoin-project/venus/issues/3955
const onChainMessageBase = int64(0)
const onChainMessagePerByte = int64(2)

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
		buf := new(bytes.Buffer)
		err := smsg.Message.MarshalCBOR(buf)
		if err != nil {
			return errors.Wrapf(err, "failed to calculate message size")
		}
		msgLen = buf.Len()
	} else {
		buf := new(bytes.Buffer)
		err := smsg.MarshalCBOR(buf)
		if err != nil {
			return errors.Wrapf(err, "failed to calculate message size")
		}
		msgLen = buf.Len()
	}
	return v.validateMessageSyntaxShared(ctx, msg, int64(msgLen))
}

// ValidateUnsignedMessageSyntax validates unisigned message syntax and state-independent invariants.
// Used for bls messages included in blocks.
func (v *DefaultMessageSyntaxValidator) ValidateUnsignedMessageSyntax(ctx context.Context, msg *types.UnsignedMessage) error {
	buf := new(bytes.Buffer)
	err := msg.MarshalCBOR(buf)
	if err != nil {
		return errors.Wrapf(err, "failed to calculate message size")
	}
	return v.validateMessageSyntaxShared(ctx, msg, int64(buf.Len()))
}

func (v *DefaultMessageSyntaxValidator) validateMessageSyntaxShared(ctx context.Context, msg *types.UnsignedMessage, msgLen int64) error {
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

	if msg.GasFeeCap.LessThan(types.ZeroFIL) {
		invGasPriceNegativeCt.Inc(ctx, 1)
		return fmt.Errorf("negative gas price %s: %s", msg.GasFeeCap, msg)
	}
	// The minimum gas limit ensures the sender has enough balance to pay for inclusion of the message in the Chain
	// *at all*. Without this, a message could hit out-of-gas but the sender pay nothing.
	// NOTE(anorth): this check has been moved to execution time, and the miner is penalized for including
	// such a message. We can probably remove this.
	minMsgGas := onChainMessageBase + onChainMessagePerByte*msgLen
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
	GetHead() *types.TipSet
	GetTipSet(types.TipSetKey) (*types.TipSet, error)
	AccountView(ts *types.TipSet) (state.AccountView, error)
}

func NewMessageSignatureValidator(api signatureValidatorAPI) *MessageSignatureValidator {
	return &MessageSignatureValidator{
		api: api,
	}
}

// Validate validates the signed message signature. Errors probably mean the
//  validation failed, but possibly indicate a failure to retrieve state.
func (v *MessageSignatureValidator) Validate(ctx context.Context, smsg *types.SignedMessage) error {
	head := v.api.GetHead()
	view, err := v.api.AccountView(head)
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
