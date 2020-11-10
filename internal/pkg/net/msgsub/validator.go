package msgsub

import (
	"context"

	"github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-pubsub"

	"github.com/filecoin-project/venus/internal/pkg/consensus"
	"github.com/filecoin-project/venus/internal/pkg/metrics"
	"github.com/filecoin-project/venus/internal/pkg/types"
)

var messageTopicLogger = log.Logger("net/message_validator")
var mDecodeMsgFail = metrics.NewInt64Counter("net/pubsub_message_decode_failure", "Number of messages that fail to decode seen on message pubsub channel")
var mInvalidMsg = metrics.NewInt64Counter("net/pubsub_invalid_message", "Number of messages that fail syntax validation seen on message pubsub channel")

// MessageTopicValidator may be registered on go-libp3p-pubsub to validate msgsub payloads.
type MessageTopicValidator struct {
	validator pubsub.Validator
	opts      []pubsub.ValidatorOpt
}

// NewMessageTopicValidator returns a MessageTopicValidator using the input
// signature and syntax validators.
func NewMessageTopicValidator(syntaxVal consensus.MessageSyntaxValidator, sigVal *consensus.MessageSignatureValidator, opts ...pubsub.ValidatorOpt) *MessageTopicValidator {
	return &MessageTopicValidator{
		opts: opts,
		validator: func(ctx context.Context, p peer.ID, msg *pubsub.Message) bool {
			unmarshaled := &types.SignedMessage{}
			if err := unmarshaled.Unmarshal(msg.GetData()); err != nil {
				messageTopicLogger.Debugf("message from peer: %s failed to decode: %s", p.String(), err.Error())
				mDecodeMsgFail.Inc(ctx, 1)
				return false
			}
			if err := syntaxVal.ValidateSignedMessageSyntax(ctx, unmarshaled); err != nil {
				mCid, _ := unmarshaled.Cid()
				messageTopicLogger.Debugf("message %s from peer: %s failed to syntax validate: %s", mCid.String(), p.String(), err.Error())
				mInvalidMsg.Inc(ctx, 1)
				return false
			}
			if err := sigVal.Validate(ctx, unmarshaled); err != nil {
				mCid, _ := unmarshaled.Cid()
				messageTopicLogger.Debugf("message %s from peer: %s failed to signature validate: %s", mCid.String(), p.String(), err.Error())
				mInvalidMsg.Inc(ctx, 1)
				return false
			}
			return true
		},
	}
}

func (mtv *MessageTopicValidator) Topic(network string) string {
	return Topic(network)
}

func (mtv *MessageTopicValidator) Validator() pubsub.Validator {
	return mtv.validator
}

func (mtv *MessageTopicValidator) Opts() []pubsub.ValidatorOpt {
	return mtv.opts
}
