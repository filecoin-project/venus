package impl

import (
	"context"
	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	logging "gx/ipfs/QmcuXC5cxs79ro2cUuHs4HQ2bkDLJUYokwL8aivcX6HW3C/go-log"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/api2"
	"github.com/filecoin-project/go-filecoin/message"
	"github.com/filecoin-project/go-filecoin/types"
)

type plumbingAPI struct {
	logger logging.EventLogger

	msgSender *message.Sender
}

// Assert that plumbingAPI fullfills the api.Plumbing interface.
var _ api2.Plumbing = (*plumbingAPI)(nil)

// New constructs a new instance of the API.
func New(msgSender *message.Sender) *plumbingAPI {
	return &plumbingAPI{
		logger: logging.Logger("api2"),

		msgSender: msgSender,
	}
}

// MessageSend implements MessageSend from api2.Message.
func (p *plumbingAPI) MessageSend(ctx context.Context, from, to address.Address, value *types.AttoFIL, method string, params ...interface{}) (cid.Cid, error) {
	return p.msgSender.Send(ctx, from, to, value, method, params...)
}
