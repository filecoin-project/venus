package msg

import (
	"context"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/ipfs/go-cid"
)

// Sender is plumbing implementation that knows how to send a message.
type Sender struct {
	outbox *core.Outbox
}

// Send sends a message. If gossip is false, the message is not broadcast to the network, but included in this
// node's next mined block. For more information, see api.go .
func (s *Sender) Send(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, gossip bool, params ...interface{}) (out cid.Cid, err error) {
	return s.outbox.Send(ctx, from, to, value, gasPrice, gasLimit, method, gossip, params...)

}
