package node_api

import (
	"context"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

type AddressAPI struct {
	api *API

	addrs *AddrsAPI
}

func NewAddressAPI(api *API) *AddressAPI {
	return &AddressAPI{
		api:   api,
		addrs: NewAddrsAPI(api),
	}
}

func (api *AddressAPI) Addrs() api.Addrs {
	return api.addrs
}

func (api *AddressAPI) Balance(ctx context.Context, addr types.Address) (*types.AttoFIL, error) {
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

type AddrsAPI struct {
	api *API
}

func NewAddrsAPI(api *API) *AddrsAPI {
	return &AddrsAPI{api: api}
}

func (api *AddrsAPI) New(ctx context.Context) (types.Address, error) {
	return api.api.node.NewAddress()
}

func (api *AddrsAPI) Ls(ctx context.Context) ([]types.Address, error) {
	return api.api.node.Wallet.Addresses(), nil
}

func (api *AddrsAPI) Lookup(ctx context.Context, addr types.Address) (peer.ID, error) {
	id, err := api.api.node.Lookup.GetPeerIDByMinerAddress(ctx, addr)
	if err != nil {
		return peer.ID(""), errors.Wrapf(err, "failed to find miner with address %s", addr.String())
	}

	return id, nil
}
