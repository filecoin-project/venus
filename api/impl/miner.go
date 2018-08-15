package impl

import (
	"context"

	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	"gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/types"
)

type nodeMiner struct {
	api *nodeAPI
}

func newNodeMiner(api *nodeAPI) *nodeMiner {
	return &nodeMiner{api: api}
}

func (api *nodeMiner) Create(ctx context.Context, fromAddr types.Address, pledge *types.BytesAmount, pid peer.ID, collateral *types.AttoFIL) (types.Address, error) {
	nd := api.api.node

	if err := setDefaultFromAddr(&fromAddr, nd); err != nil {
		return types.Address{}, err
	}

	if pid == "" {
		pid = nd.Host.ID()
	}

	res, err := nd.CreateMiner(ctx, fromAddr, *pledge, pid, *collateral)
	if err != nil {
		return types.Address{}, err
	}

	return *res, nil
}

func (api *nodeMiner) UpdatePeerID(ctx context.Context, fromAddr, minerAddr types.Address, newPid peer.ID) (*cid.Cid, error) {
	return api.api.Message().Send(
		ctx,
		fromAddr,
		minerAddr,
		nil,
		"updatePeerID",
		newPid,
	)
}

func (api *nodeMiner) AddAsk(ctx context.Context, fromAddr, minerAddr types.Address, size *types.BytesAmount, price *types.AttoFIL) (*cid.Cid, error) {
	return api.api.Message().Send(
		ctx,
		fromAddr,
		minerAddr,
		nil,
		"addAsk",
		price, size,
	)
}
