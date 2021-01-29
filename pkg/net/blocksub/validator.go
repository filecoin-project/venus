package blocksub

import (
	"bytes"
	"context"
	"github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/metrics"
)

var blockTopicLogger = log.Logger("net/block_validator")
var mDecodeBlkFail = metrics.NewInt64Counter("net/pubsub_block_decode_failure", "Number of blocks that fail to decode seen on block pubsub channel")
var mInvalidBlk = metrics.NewInt64Counter("net/pubsub_invalid_block", "Number of blocks that fail syntax validation seen on block pubsub channel")

// BlockTopicValidator may be registered on go-libp2p-pubsub to validate blocksub messages.
type BlockTopicValidator struct {
	validator pubsub.Validator
	opts      []pubsub.ValidatorOpt
}

type BlockHeaderValidator interface {
	ValidateBlockHeader(ctx context.Context, blk *block.Block) error
}

// NewBlockTopicValidator retruns a BlockTopicValidator using `bv` for message validation
func NewBlockTopicValidator(bv BlockHeaderValidator, opts ...pubsub.ValidatorOpt) *BlockTopicValidator {
	return &BlockTopicValidator{
		opts: opts,
		validator: func(ctx context.Context, p peer.ID, msg *pubsub.Message) bool {
			var bm block.BlockMsg
			err := bm.UnmarshalCBOR(bytes.NewReader(msg.GetData()))
			if err != nil {
				blockTopicLogger.Warnf("failed to decode blocksub payload from peer %s: %s", p.String(), err.Error())
				mDecodeBlkFail.Inc(ctx, 1)
				return false
			}
			//todo validate block
			if err := bv.ValidateBlockHeader(ctx, bm.Header); err != nil {
				blockTopicLogger.Warnf("failed to validate block %s from peer %s: %s", bm.Header.Cid().String(), p.String(), err.Error())
				mInvalidBlk.Inc(ctx, 1)
				return false
			}
			return true
		},
	}
}

func (btv *BlockTopicValidator) Topic(network string) string {
	return Topic(network)
}

func (btv *BlockTopicValidator) Validator() pubsub.Validator {
	return btv.validator
}

func (btv *BlockTopicValidator) Opts() []pubsub.ValidatorOpt {
	return btv.opts
}
