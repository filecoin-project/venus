package impl

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"
)

type nodeAddress struct {
	api *nodeAPI
}

func newNodeAddress(api *nodeAPI) *nodeAddress {
	return &nodeAddress{
		api: api,
	}
}

func (api *nodeAddress) Import(ctx context.Context, kinfos []*types.KeyInfo) ([]address.Address, error) {
	nd := api.api.node

	dsb := nd.Wallet.Backends(wallet.DSBackendType)
	if len(dsb) != 1 {
		return nil, fmt.Errorf("expected exactly one datastore wallet backend")
	}

	imp, ok := dsb[0].(wallet.Importer)
	if !ok {
		return nil, fmt.Errorf("datastore backend wallets should implement importer")
	}

	var out []address.Address
	for _, ki := range kinfos {
		if err := imp.ImportKey(ki); err != nil {
			return nil, err
		}

		a, err := ki.Address()
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
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
