package node

import (
	"context"

	"github.com/filecoin-project/go-filecoin/types"
)

// BlocksTopic is the string topic for the blocks pubsub channel
const BlocksTopic = "/fil/blocks"

// AddNewBlock adds a new locally crafted block and announces it to the network.
func (node *Node) AddNewBlock(ctx context.Context, b *types.Block) error {
	if _, err := node.ChainMgr.ProcessNewBlock(ctx, b); err != nil {
		return err
	}
	return node.PubSub.Publish(BlocksTopic, b.ToNode().RawData())
}

func (node *Node) handleBlockSubscription() {
	ctx := context.TODO()
	for {
		msg, err := node.BlockSub.Next(ctx)
		if err != nil {
			log.Errorf("blocksub.Next(): %s", err)
			return
		}
		log.Error("got a block!")

		// ignore messages from ourself
		if msg.GetFrom() == node.Host.ID() {
			continue
		}

		blk, err := types.DecodeBlock(msg.GetData())
		if err != nil {
			log.Errorf("got bad block data: %s", err)
			continue
		}

		if res, err := node.ChainMgr.ProcessNewBlock(ctx, blk); err != nil {
			log.Errorf("processing block from network: %s", err)
			continue
		} else {
			log.Error("process blocks returned: ", res)
		}
	}
}
