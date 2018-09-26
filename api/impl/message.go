package impl

import (
	"context"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/types"
)

type nodeMessage struct {
	api *nodeAPI
}

func newNodeMessage(api *nodeAPI) *nodeMessage {
	return &nodeMessage{api: api}
}

func (api *nodeMessage) Send(ctx context.Context, from, to address.Address, val *types.AttoFIL, method string, params ...interface{}) (*cid.Cid, error) {
	nd := api.api.node

	if err := setDefaultFromAddr(&from, nd); err != nil {
		return nil, err
	}

	return nd.SendMessage(ctx, from, to, val, method, params...)
}

// Query requests information from an actor in the local state.
// The information is based on the current heaviest tipset. Answers may change as new blocks are received.
func (api *nodeMessage) Query(ctx context.Context, from, to address.Address, method string, params ...interface{}) ([][]byte, *exec.FunctionSignature, error) {
	nd := api.api.node

	// get correct from address
	if err := setDefaultFromAddr(&from, nd); err != nil {
		return nil, nil, err
	}

	// get signature for return value
	signature, err := nd.GetSignature(ctx, to, method)
	if err != nil && err != node.ErrNoMethod {
		return nil, nil, errors.Wrap(err, "unable to determine return type")
	}

	// encode the parameters
	encodedParams, err := abi.ToEncodedValues(params...)
	if err != nil {
		return nil, nil, err
	}

	// make the request
	retVals, ec, err := nd.CallQueryMethod(ctx, to, method, encodedParams, &from)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed executing query method")
	}

	if ec != 0 {
		return nil, nil, errors.Errorf("Non-zero return from query: %d", ec)
	}

	return retVals, signature, nil
}

func (api *nodeMessage) Wait(ctx context.Context, msgCid *cid.Cid, cb func(blk *types.Block, msg *types.SignedMessage, receipt *types.MessageReceipt, signature *exec.FunctionSignature) error) error {
	nd := api.api.node

	return nd.ChainMgr.WaitForMessage(ctx, msgCid, func(blk *types.Block, msg *types.SignedMessage, receipt *types.MessageReceipt) error {
		signature, err := nd.GetSignature(ctx, msg.To, msg.Method)
		if err != nil && err != node.ErrNoMethod {
			return errors.Wrap(err, "unable to determine return type")
		}

		return cb(blk, msg, receipt, signature)
	})
}
