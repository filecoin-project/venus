package f3

import (
	"context"

	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/app/submodule/network"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/vf3"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/api/f3"
	logging "github.com/ipfs/go-log"
)

var log = logging.Logger("f3")

type F3Submodule struct {
	F3 *vf3.F3
}

func NewF3Submodule(ctx context.Context,
	repo repo.Repo,
	chain *chain.ChainSubmodule,
	network *network.NetworkSubmodule,
	walletAPI v1api.IWallet,
) (*F3Submodule, error) {
	netConf := repo.Config().NetworkParams
	if !netConf.F3Enabled {
		return &F3Submodule{
			F3: nil,
		}, nil
	}
	m, err := vf3.New(ctx, vf3.F3Params{
		ManifestServerID: netConf.ManifestServerID,
		PubSub:           network.Pubsub,
		Host:             network.Host,
		ChainStore:       chain.ChainReader,
		StateManager:     chain.Stmgr,
		Datastore:        repo.ChainDatastore(),
		Wallet:           walletAPI,
		ManifestProvider: vf3.NewManifestProvider(network.NetworkName, chain.ChainReader, chain.Stmgr, network.Pubsub, netConf),
	})
	if err != nil {
		return nil, err
	}

	return &F3Submodule{m}, nil
}

func (m *F3Submodule) API() f3.F3 {
	return &f3API{
		f3module: m,
	}
}

func (m *F3Submodule) V0API() f3.F3 {
	return &f3API{
		f3module: m,
	}
}
