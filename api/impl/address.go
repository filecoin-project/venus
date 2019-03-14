package impl

import (
	"context"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

type nodeAddress struct {
	api *nodeAPI
}

func newNodeAddress(api *nodeAPI) *nodeAddress {
	return &nodeAddress{
		api: api,
	}
}

func (api *nodeAddress) Export(ctx context.Context, addrs []address.Address) ([]*types.KeyInfo, error) {
	nd := api.api.node

	out := make([]*types.KeyInfo, len(addrs))
	for i, addr := range addrs {
		bck, err := nd.Wallet.Find(addr)
		if err != nil {
			return nil, err
		}

		ki, err := bck.GetKeyInfo(addr)
		if err != nil {
			return nil, err
		}
		out[i] = ki
	}

	return out, nil
}
