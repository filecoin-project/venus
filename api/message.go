package api

import (
	"context"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
)

// Message is the interface that defines methods to manage various message operations,
// like sending and awaiting mined ones.
type Message interface {
	Send(ctx context.Context, from, to address.Address, val *types.AttoFIL, method string, params ...interface{}) (*cid.Cid, error)
	Query(ctx context.Context, from, to address.Address, method string, params ...interface{}) ([][]byte, *exec.FunctionSignature, error)
	Wait(ctx context.Context, msgCid *cid.Cid, cb func(blk *types.Block, msg *types.SignedMessage, receipt *types.MessageReceipt, signature *exec.FunctionSignature) error) error
}
