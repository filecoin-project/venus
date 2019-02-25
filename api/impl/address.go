package impl

import (
	"context"
	"encoding/json"
	"fmt"

	"gx/ipfs/QmQmhotPUzVrMEWNK3x1R5jQ5ZHWyL7tVUrmRPjrBrvyCb/go-ipfs-files"
	"gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"
)

type nodeAddress struct {
	api *nodeAPI

	addrs *nodeAddrs
}

func newNodeAddress(api *nodeAPI) *nodeAddress {
	return &nodeAddress{
		api:   api,
		addrs: newNodeAddrs(api),
	}
}

func (api *nodeAddress) Addrs() api.Addrs {
	return api.addrs
}

// WalletSerializeResult is the type wallet export and import return and expect.
type WalletSerializeResult struct {
	KeyInfo []*types.KeyInfo
}

type nodeAddrs struct {
	api *nodeAPI
}

func newNodeAddrs(api *nodeAPI) *nodeAddrs {
	return &nodeAddrs{api: api}
}

func (api *nodeAddrs) New(ctx context.Context) (address.Address, error) {
	return api.api.node.NewAddress()
}

func (api *nodeAddrs) Ls(ctx context.Context) ([]address.Address, error) {
	return api.api.node.Wallet.Addresses(), nil
}

func (api *nodeAddrs) Lookup(ctx context.Context, addr address.Address) (peer.ID, error) {
	id, err := api.api.node.Lookup().GetPeerIDByMinerAddress(ctx, addr)
	if err != nil {
		return peer.ID(""), errors.Wrapf(err, "failed to find miner with address %s", addr.String())
	}

	return id, nil
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
