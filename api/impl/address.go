package impl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmPJxxDsX2UbchSHobbYuvz7qnyJTFKvaKMzE2rZWJ4x5B/go-libp2p-peer"
	"gx/ipfs/QmZMWMvWMVKCbHetJ4RgndbuEF1io2UpUxwQwtNjtYPzSC/go-ipfs-files"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/state"
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

func (api *nodeAddress) Balance(ctx context.Context, addr address.Address) (*types.AttoFIL, error) {
	fcn := api.api.node

	tree, err := fcn.ChainReader.LatestState(ctx)
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
	var kinfos []*types.KeyInfo
	for {
		fi, err := f.NextFile()
		switch err {
		case io.EOF:
			return kinfos, nil
		case nil: // noop
		default:
			return nil, err
		}

		var ki types.KeyInfo
		if err := json.NewDecoder(fi).Decode(&ki); err != nil {
			return nil, err
		}

		kinfos = append(kinfos, &ki)
	}
}
