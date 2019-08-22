package net

import (
	"context"

	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-pubsub"

	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/types"
)

var blockTopicLogger = logging.Logger("net/block_validator")

// BlockTopicValidator may be registered on go-libp2p-pubsub to validate pubsub messages on the
// BlockTopic.
type BlockTopicValidator struct {
	validator pubsub.Validator
	opts      []pubsub.ValidatorOpt
}

// NewBlockTopicValidator retruns a BlockTopicValidator using `bv` for message validation
func NewBlockTopicValidator(bv consensus.BlockSyntaxValidator, opts ...pubsub.ValidatorOpt) *BlockTopicValidator {
	return &BlockTopicValidator{
		opts: opts,
		validator: func(ctx context.Context, p peer.ID, msg *pubsub.Message) bool {
			blk, err := types.DecodeBlock(msg.GetData())
			if err != nil {
				blockTopicLogger.Debugf("block from peer: %s failed to decode: %s", p.String(), err.Error())
				return false
			}
			if err := bv.ValidateSyntax(ctx, blk); err != nil {
				blockTopicLogger.Debugf("block: %s from peer: %s failed to validate: %s", blk.Cid().String(), p.String(), err.Error())
				return false
			}
			return true
		},
	}
}

// Topic returns the topic string BlockTopic
func (btv *BlockTopicValidator) Topic() string {
	return BlockTopic
}

// Validator returns a validation method matching the Validator pubsub function signature.
func (btv *BlockTopicValidator) Validator() pubsub.Validator {
	return btv.validator
}

// Opts returns the pubsub ValidatorOpts the BlockTopicValidator is configured to use.
func (btv *BlockTopicValidator) Opts() []pubsub.ValidatorOpt {
	return btv.opts
}
