package commands

import (
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/types"
)

func addressWithDefault(rawAddr interface{}, n *node.Node) (types.Address, error) {
	stringAddr, _ := rawAddr.(string)
	var addr types.Address
	var err error
	if stringAddr != "" {
		addr, err = types.ParseAddress(stringAddr)
		if err != nil {
			return "", err
		}
	} else {
		addr, err = n.Wallet.GetDefaultAddress()
		if err != nil {
			return "", err
		}
	}

	return addr, nil
}

func waitForMessage(n *node.Node, msgCid *cid.Cid, cb func(*types.Block, *types.Message, *types.MessageReceipt)) {
	ch := n.ChainMgr.BestBlockPubSub.Sub(core.BlockTopic)
	defer n.ChainMgr.BestBlockPubSub.Unsub(ch, core.BlockTopic)

	for blkRaw := range ch {
		blk := blkRaw.(*types.Block)
		for i, msg := range blk.Messages {
			c, err := msg.Cid()
			if err != nil {
				continue
			}
			if c.Equals(msgCid) {
				cb(blk, msg, blk.MessageReceipts[i])
				return
			}
		}
	}
}
