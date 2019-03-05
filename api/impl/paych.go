package impl

import (
	"context"

	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/porcelain"
)

type nodePaych struct {
	api          *nodeAPI
	porcelainAPI *porcelain.API
}

func newNodePaych(api *nodeAPI, porcelainAPI *porcelain.API) *nodePaych {
	return &nodePaych{api: api, porcelainAPI: porcelainAPI}
}

func (np *nodePaych) Ls(ctx context.Context, fromAddr, payerAddr address.Address) (map[string]*paymentbroker.PaymentChannel, error) {
	nd := np.api.node

	if err := setDefaultFromAddr(&fromAddr, nd); err != nil {
		return nil, err
	}

	if payerAddr == (address.Address{}) {
		payerAddr = fromAddr
	}

	values, _, err := np.porcelainAPI.MessageQuery(
		ctx,
		fromAddr,
		address.PaymentBrokerAddress,
		"ls",
		payerAddr,
	)
	if err != nil {
		return nil, err
	}

	var channels map[string]*paymentbroker.PaymentChannel

	if err := cbor.DecodeInto(values[0], &channels); err != nil {
		return nil, err
	}

	return channels, nil
}
