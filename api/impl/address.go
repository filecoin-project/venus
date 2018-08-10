package impl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmdE4gMduCKCGAcczM2F5ioYDfdeKuPix138wrES1YSr7f/go-ipfs-cmdkit/files"
	"gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"
)

type NodeAddress struct {
	api *NodeAPI

	addrs *NodeAddrs
}

func NewNodeAddress(api *NodeAPI) *NodeAddress {
	return &NodeAddress{
		api:   api,
		addrs: NewNodeAddrs(api),
	}
}

func (api *NodeAddress) Addrs() api.Addrs {
	return api.addrs
}

func (api *NodeAddress) Balance(ctx context.Context, addr types.Address) (*types.AttoFIL, error) {
	fcn := api.api.node
	ts := fcn.ChainMgr.GetHeaviestTipSet()
	if len(ts) == 0 {
		return types.ZeroAttoFIL, ErrHeaviestTipSetNotFound
	}

	tree, _, err := fcn.ChainMgr.State(ctx, ts.ToSlice())
	if err != nil {
		return types.ZeroAttoFIL, err
	}

	act, err := tree.GetActor(ctx, addr)
	if err != nil {
		if state.IsActorNotFoundError(err) {
			// if the account doesn't exit, the balance should be zero
			return types.NewAttoFILFromFIL(0), nil
		}

		return types.ZeroAttoFIL, err
	}

	return act.Balance, nil
}

type NodeAddrs struct {
	api *NodeAPI
}

func NewNodeAddrs(api *NodeAPI) *NodeAddrs {
	return &NodeAddrs{api: api}
}

func (api *NodeAddrs) New(ctx context.Context) (types.Address, error) {
	return api.api.node.NewAddress()
}

func (api *NodeAddrs) Ls(ctx context.Context) ([]types.Address, error) {
	return api.api.node.Wallet.Addresses(), nil
}

func (api *NodeAddrs) Lookup(ctx context.Context, addr types.Address) (peer.ID, error) {
	id, err := api.api.node.Lookup.GetPeerIDByMinerAddress(ctx, addr)
	if err != nil {
		return peer.ID(""), errors.Wrapf(err, "failed to find miner with address %s", addr.String())
	}

	return id, nil
}

func (api *NodeAddress) Import(ctx context.Context, f files.File) error {
	nd := api.api.node

	kinfos, err := parseKeyInfos(f)
	if err != nil {
		return err
	}

	dsb := nd.Wallet.Backends(wallet.DSBackendType)
	if len(dsb) != 1 {
		return fmt.Errorf("expected exactly one datastore wallet backend")
	}

	imp, ok := dsb[0].(wallet.Importer)
	if !ok {
		return fmt.Errorf("datastore backend wallets should implement importer")
	}

	for _, ki := range kinfos {
		if err := imp.ImportKey(ki); err != nil {
			return err
		}
	}
	return nil
}

func (api *NodeAddress) Export(ctx context.Context, addrs []types.Address) ([]*types.KeyInfo, error) {
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
	var kinfos []*types.KeyInfo
	for {
		fi, err := f.NextFile()
		switch err {
		case io.EOF:
			return kinfos, nil
		default:
			return nil, err
		case nil:
		}

		var ki types.KeyInfo
		if err := json.NewDecoder(fi).Decode(&ki); err != nil {
			return nil, err
		}

		kinfos = append(kinfos, &ki)
	}
}
