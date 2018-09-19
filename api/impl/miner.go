package impl

import (
	"context"

	"gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

type nodeMiner struct {
	api *nodeAPI
}

func newNodeMiner(api *nodeAPI) *nodeMiner {
	return &nodeMiner{api: api}
}

func (api *nodeMiner) Create(ctx context.Context, fromAddr address.Address, pledge *types.BytesAmount, pid peer.ID, collateral *types.AttoFIL) (address.Address, error) {
	nd := api.api.node

	if err := setDefaultFromAddr(&fromAddr, nd); err != nil {
		return address.Address{}, err
	}

	if pid == "" {
		pid = nd.Host.ID()
	}

	res, err := nd.CreateMiner(ctx, fromAddr, *pledge, pid, *collateral)
	if err != nil {
		return address.Address{}, err
	}

	return *res, nil
}

func (api *nodeMiner) UpdatePeerID(ctx context.Context, fromAddr, minerAddr address.Address, newPid peer.ID) (*cid.Cid, error) {
	return api.api.Message().Send(
		ctx,
		fromAddr,
		minerAddr,
		nil,
		"updatePeerID",
		newPid,
	)
}

func (api *nodeMiner) AddAsk(ctx context.Context, fromAddr, minerAddr address.Address, size *types.BytesAmount, price *types.AttoFIL) (*cid.Cid, error) {
	return api.api.Message().Send(
		ctx,
		fromAddr,
		minerAddr,
		nil,
		"addAsk",
		price, size,
	)
}

func (api *nodeMiner) GetOwner(ctx context.Context, minerAddr address.Address) (address.Address, error) {
	bytes, _, err := api.api.Message().Query(
		ctx,
		address.Address{},
		minerAddr,
		"getOwner",
	)
	if err != nil {
		return address.Address{}, err
	}

	return address.NewFromBytes(bytes[0])
}
