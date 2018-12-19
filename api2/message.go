package api2

import (
	"context"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

// Message is the message-related Filecoin plumbing interface.
type Message interface {
	// MessageSend enqueues a message in the message pool and broadcasts it to the network.
	MessageSend(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasCost, method string, params ...interface{}) (cid.Cid, error)

	// MessageWait invokes the callback when a message with the given cid appears on chain,
	// or when it finds the message already on chain. It is possible that both an error
	// is returned and the callback is invoked, eg if an error was encountered trying to
	// find the block in the block history but it suddenly appears in a newly mined block.
	// It keeps trying until the context is canceled.
	MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error
}
