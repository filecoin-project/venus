package impl

import (
	"context"

	"github.com/libp2p/go-libp2p-peer"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/types"
)

type nodeMiner struct {
	api          *nodeAPI
	porcelainAPI *porcelain.API
}

func newNodeMiner(api *nodeAPI, porcelainAPI *porcelain.API) *nodeMiner {
	return &nodeMiner{api: api, porcelainAPI: porcelainAPI}
}

func (nm *nodeMiner) Create(ctx context.Context, fromAddr address.Address, gasPrice types.AttoFIL, gasLimit types.GasUnits, pledge uint64, pid peer.ID, collateral *types.AttoFIL) (address.Address, error) {
	nd := nm.api.node

	if err := setDefaultFromAddr(&fromAddr, nd); err != nil {
		return address.Address{}, err
	}

	if pid == "" {
		pid = nd.Host().ID()
	}

	res, err := nd.CreateMiner(ctx, fromAddr, gasPrice, gasLimit, pledge, pid, collateral)
	if err != nil {
		return address.Address{}, errors.Wrap(err, "Could not create miner. Please consult the documentation to setup your wallet and genesis block correctly")
	}

	return *res, nil
}
