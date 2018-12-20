package impl

import (
	"context"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/api2/impl/mthdsigapi"
	"github.com/filecoin-project/go-filecoin/exec"
)

type nodeMessage struct {
	api *nodeAPI
}

func newNodeMessage(api *nodeAPI) *nodeMessage {
	return &nodeMessage{api: api}
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
	sigGetter := mthdsigapi.NewGetter(nd.ChainReader)
	sig, err := sigGetter.Get(ctx, to, method)
	if err != nil {
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
		return nil, nil, errors.Errorf("non-zero return from query: %d", ec)
	}

	return retVals, sig, nil
}
