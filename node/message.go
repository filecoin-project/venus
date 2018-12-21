package node

import (
	"context"
	"encoding/json"

	"gx/ipfs/Qmc3BYVGtLs8y3p4uVpARWyo3Xk2oCBFF1AhYUVMPWgwUK/go-libp2p-pubsub"

	"github.com/filecoin-project/go-filecoin/types"
)

func (node *Node) processMessage(ctx context.Context, pubSubMsg *pubsub.Message) (err error) {
	ctx = log.Start(ctx, "Node.processMessage")
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	unmarshaled := &types.SignedMessage{}
	if err := unmarshaled.Unmarshal(pubSubMsg.GetData()); err != nil {
		return err
	}
	log.SetTag(ctx, "message", unmarshaled)

	// TODO debugstring
	cid, err := unmarshaled.Cid()
	if err != nil {
		log.Error("Error getting cid for new message %v", unmarshaled)
	} else {
		js, err := json.MarshalIndent(unmarshaled, "", "  ")
		if err != nil {
			log.Error("Error json encoding new message")
		} else {
			log.Debug("Received new message %v, contents: %v", cid, string(js))
		}
	}

	_, err = node.MsgPool.Add(unmarshaled)
	return err
}
