package node

import (
	"context"

	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/types"
)

var BlocksTopic = "/fil/blocks"
var MessageTopic = "/fil/msgs"

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

func (node *Node) AddNewMessage(ctx context.Context, msg *types.Message) error {
	if err := node.MsgPool.Add(msg); err != nil {
		return err
	}

	msgdata, err := msg.Marshal()
	if err != nil {
		return err
	}

	return node.PubSub.Publish(MessageTopic, msgdata)
}

func (node *Node) handleMessageSubscription() {
	ctx := context.TODO()
	for {
		msg, err := node.BlockSub.Next(ctx)
		if err != nil {
			log.Errorf("blocksub.Next(): %s", err)
			return
		}

		// ignore messages from ourself
		if msg.GetFrom() == node.Host.ID() {
			continue
		}

		if err := node.handleMessage(msg.GetData()); err != nil {
			log.Error(err)
		}
	}
}

func (node *Node) handleMessage(msgdata []byte) error {
	var m types.Message
	if err := m.Unmarshal(msgdata); err != nil {
		return errors.Wrap(err, "got bad message data")
	}

	if err := node.MsgPool.Add(&m); err != nil {
		return errors.Wrapf(err, "processing message from network")
	}
	return nil
}
