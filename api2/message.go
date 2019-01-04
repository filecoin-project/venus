package api2

import (
	"context"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

// Message is the message-related Filecoin plumbing interface.
type Message interface {
	// MessageSend sends a message. It uses the default from address if none is given and signs the
	// message using the wallet. This call "sends" in the sense that it enqueues the
	// message in the msg pool and broadcasts it to the network; it does not wait for the
	// message to go on chain.
	MessageSend(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasCost, method string, params ...interface{}) (cid.Cid, error)

	// MessageWait invokes the callback when a message with the given cid appears on chain.
	// It will find the message in both the case that it is already on chain and
	// the case that it appears in a newly mined block. An error is returned if one is
	// encountered or if the context is canceled. Otherwise, it waits forever for the message
	// to appear on chain.
	MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error
}
