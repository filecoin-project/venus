package impl

import (
	"context"
	"math/big"

	"gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
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

func (api *nodeMiner) Create(ctx context.Context, fromAddr address.Address, pledge uint64, pid peer.ID, collateral *types.AttoFIL) (address.Address, error) {
	nd := api.api.node

	if err := setDefaultFromAddr(&fromAddr, nd); err != nil {
		return address.Address{}, err
	}

	if pid == "" {
		pid = nd.Host().ID()
	}

	res, err := nd.CreateMiner(ctx, fromAddr, pledge, pid, collateral)
	if err != nil {
		return address.Address{}, errors.Wrap(err, "Could not create miner. Please consult the documentation to setup your wallet and genesis block correctly")
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

func (api *nodeMiner) AddAsk(ctx context.Context, fromAddr, minerAddr address.Address, price *types.AttoFIL, expiry *big.Int) (*cid.Cid, error) {
	return api.api.Message().Send(
		ctx,
		fromAddr,
		minerAddr,
		nil,
		"addAsk",
		price,
		expiry,
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

func (api *nodeMiner) GetPower(ctx context.Context, minerAddr address.Address) (*big.Int, error) {
	bytes, _, err := api.api.Message().Query(
		ctx,
		address.Address{},
		minerAddr,
		"getPower",
	)
	if err != nil {
		return nil, err
	}

	power := big.NewInt(0).SetBytes(bytes[0])

	return power, nil
}

func (api *nodeMiner) GetPledge(ctx context.Context, minerAddr address.Address) (*big.Int, error) {
	bytes, _, err := api.api.Message().Query(
		ctx,
		address.Address{},
		minerAddr,
		"getPledge",
	)
	if err != nil {
		return nil, err
	}

	power := big.NewInt(0).SetBytes(bytes[0])

	return power, nil
}

func (api *nodeMiner) GetTotalPower(ctx context.Context) (*big.Int, error) {
	bytes, _, err := api.api.Message().Query(
		ctx,
		address.Address{},
		address.StorageMarketAddress,
		"getTotalStorage",
	)
	if err != nil {
		return nil, err
	}

	power := big.NewInt(0).SetBytes(bytes[0])

	return power, nil
}
