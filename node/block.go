package node

import (
	"context"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/Qmc3BYVGtLs8y3p4uVpARWyo3Xk2oCBFF1AhYUVMPWgwUK/go-libp2p-pubsub"

	"github.com/filecoin-project/go-filecoin/types"
)

// BlockTopic is the pubsub topic identifier on which new blocks are announced.
const BlockTopic = "/fil/blocks"

// AddNewBlock receives a newly mined block and stores, validates and propagates it to the network.
func (node *Node) AddNewBlock(ctx context.Context, b *types.Block) (err error) {
	ctx = log.Start(ctx, "Node.AddNewBlock")
	log.SetTag(ctx, "block", b)
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	// Put block in storage wired to an exchange so this node and other
	// nodes can fetch it.
	log.Debugf("Putting block in bitswap exchange: %s", b.Cid().String())
	blkCid, err := node.OnlineStore.Put(ctx, b)
	if err != nil {
		return errors.Wrap(err, "could not add new block to online storage")
	}

	log.Debugf("Syncing new block: %s", b.Cid().String())
	if err := node.Syncer.HandleNewBlocks(ctx, []cid.Cid{blkCid}); err != nil {
		return err
	}

	// TODO: should this just be a cid? Right now receivers ask to fetch
	// the block over bitswap anyway.
	return node.PubSub.Publish(BlockTopic, b.ToNode().RawData())
}

func (node *Node) processBlock(ctx context.Context, pubSubMsg *pubsub.Message) (err error) {
	ctx = log.Start(ctx, "Node.processBlock")
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	// ignore messages from ourself
	if pubSubMsg.GetFrom() == node.Host().ID() {
		return nil
	}

	blk, err := types.DecodeBlock(pubSubMsg.GetData())
	if err != nil {
		return errors.Wrap(err, "got bad block data")
	}
	log.SetTag(ctx, "block", blk)

	// // TODO debugstring
	// cid, err := unmarshaled.Cid()
	// if err != nil {
	// 	log.Error("Error getting cid for new message %v", unmarshaled)
	// } else {
	// 	js, err := json.MarshalIndent(unmarshaled, "", "  ")
	// 	if err != nil {
	// 		log.Error("Error json encoding new message")
	// 	} else {
	// 		log.Debug("Received new message %v, contents: %v", cid, string(js))
	// 	}
	// }

	err = node.Syncer.HandleNewBlocks(ctx, []cid.Cid{blk.Cid()})
	if err != nil {
		return errors.Wrap(err, "processing block from network")
	}
	return nil
}
