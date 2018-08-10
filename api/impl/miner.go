package impl

import (
	"context"

	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	"gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/node"
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
	nd := api.api.node

	if err := setDefaultFromAddr(&fromAddr, nd); err != nil {
		return nil, err
	}

	args, err := abi.ToEncodedValues(newPid)
	if err != nil {
		return nil, err
	}

	msg, err := node.NewMessageWithNextNonce(ctx, nd, fromAddr, minerAddr, nil, "updatePeerID", args)
	if err != nil {
		return nil, err
	}

	smsg, err := types.NewSignedMessage(*msg, nd.Wallet)
	if err != nil {
		return nil, err
	}

	if err := nd.AddNewMessage(ctx, smsg); err != nil {
		return nil, err
	}

	return smsg.Cid()

}

func (api *nodeMiner) AddAsk(ctx context.Context, fromAddr, minerAddr types.Address, size *types.BytesAmount, price *types.AttoFIL) (*cid.Cid, error) {
	nd := api.api.node

	if err := setDefaultFromAddr(&fromAddr, nd); err != nil {
		return nil, err
	}

	params, err := abi.ToEncodedValues(price, size)
	if err != nil {
		return nil, err
	}

	msg, err := node.NewMessageWithNextNonce(ctx, nd, fromAddr, minerAddr, nil, "addAsk", params)
	if err != nil {
		return nil, err
	}

	smsg, err := types.NewSignedMessage(*msg, nd.Wallet)
	if err != nil {
		return nil, err
	}

	if err := nd.AddNewMessage(ctx, smsg); err != nil {
		return nil, err
	}

	return smsg.Cid()
}
