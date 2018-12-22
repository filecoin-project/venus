package impl

import (
	"context"
	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	logging "gx/ipfs/QmcuXC5cxs79ro2cUuHs4HQ2bkDLJUYokwL8aivcX6HW3C/go-log"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/api2"
	"github.com/filecoin-project/go-filecoin/api2/impl/msgapi"
	"github.com/filecoin-project/go-filecoin/api2/impl/mthdsigapi"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
)

// PlumbingAPI is an implementation of api2.Plumbing.
type PlumbingAPI struct {
	logger logging.EventLogger

	sigGetter *mthdsigapi.Getter
	msgSender *msgapi.Sender
	msgWaiter *msgapi.Waiter
}

// Assert that plumbingAPI fullfills the api.Plumbing interface.
var _ api2.Plumbing = (*PlumbingAPI)(nil)

// New constructs a new instance of the API.
func New(sigGetter *mthdsigapi.Getter, msgSender *msgapi.Sender, msgWaiter *msgapi.Waiter) *PlumbingAPI {
	return &PlumbingAPI{
		logger: logging.Logger("api2"),

		sigGetter: sigGetter,
		msgSender: msgSender,
		msgWaiter: msgWaiter,
	}
}

// ActorGetSignature implements GetSignature from api2.Actor.
func (p *PlumbingAPI) ActorGetSignature(ctx context.Context, actorAddr address.Address, method string) (_ *exec.FunctionSignature, err error) {
	return p.sigGetter.Get(ctx, actorAddr, method)
}

// MessageSend implements MessageSend from api2.Message.
func (p *PlumbingAPI) MessageSend(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasCost, method string, params ...interface{}) (cid.Cid, error) {
	return p.msgSender.Send(ctx, from, to, value, gasPrice, gasLimit, method, params...)
}

// MessageWait implements MessageWait from api2.Message.
func (p *PlumbingAPI) MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	return p.msgWaiter.Wait(ctx, msgCid, cb)
}
