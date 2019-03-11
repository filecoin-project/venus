package impl

import (
	"context"
	"encoding/json"
	"fmt"

	"gx/ipfs/QmQmhotPUzVrMEWNK3x1R5jQ5ZHWyL7tVUrmRPjrBrvyCb/go-ipfs-files"

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

// WalletSerializeResult is the type wallet export and import return and expect.
type WalletSerializeResult struct {
	KeyInfo []*types.KeyInfo
}

func (api *nodeAddress) Import(ctx context.Context, f files.File) ([]address.Address, error) {
	nd := api.api.node

	kinfos, err := parseKeyInfos(f)
	if err != nil {
		return nil, err
	}

	// error if we fail to parse keyinfo from the provided file.
	if len(kinfos) == 0 {
		return nil, fmt.Errorf("no keys in wallet file")
	}

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

func parseKeyInfos(f files.File) ([]*types.KeyInfo, error) {
	var wir *WalletSerializeResult
	if err := json.NewDecoder(f).Decode(&wir); err != nil {
		return nil, err
	}
	return wir.KeyInfo, nil
}
