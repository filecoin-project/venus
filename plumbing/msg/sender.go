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

// NewSender creates a new sender plumbing
func NewSender(outbox *core.Outbox) *Sender {
	return &Sender{outbox}
}

// Send sends a message. See api description.
func (s *Sender) Send(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (out cid.Cid, err error) {
	return s.outbox.Send(ctx, from, to, value, gasPrice, gasLimit, method, params...)

}
