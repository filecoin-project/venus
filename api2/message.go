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
	MessageSend(ctx context.Context, from, to address.Address, value *types.AttoFIL, method string, params ...interface{}) (cid.Cid, error)
}
