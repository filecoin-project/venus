package node_api

import (
	"context"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

type NodeAddress struct {
	api *API

	addrs *NodeAddrs
}

func NewNodeAddress(api *API) *NodeAddress {
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
	api *API
}

func NewNodeAddrs(api *API) *NodeAddrs {
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
