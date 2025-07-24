package f3

import (
	"context"

	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/app/submodule/network"
	"github.com/filecoin-project/venus/app/submodule/syncer"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/vf3"
	"github.com/filecoin-project/venus/pkg/wallet"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
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
	walletSign wallet.WalletSignFunc,
	syncer *syncer.SyncerSubmodule,
) (*F3Submodule, error) {
	netConf := repo.Config().NetworkParams
	if !netConf.F3Enabled {
		return &F3Submodule{
			F3: nil,
		}, nil
	}
	repoPath, err := repo.Path()
	if err != nil {
		return nil, err
	}

	m, err := vf3.New(ctx, vf3.F3Params{
		PubSub:       network.Pubsub,
		Host:         network.Host,
		ChainStore:   chain.ChainReader,
		StateManager: chain.Stmgr,
		Datastore:    repo.MetaDatastore(),
		WalletSign:   walletSign,
		SyncerAPI:    syncer.API(),
		Config:       network.F3Cfg,
		RepoPath:     repoPath,
		Net:          network.API(),
	})
	if err != nil {
		return nil, err
	}

	return &F3Submodule{m}, nil
}

func (m *F3Submodule) API() v1api.IF3 {
	return &f3API{
		f3module: m,
	}
}

func (m *F3Submodule) Stop(ctx context.Context) error {
	if m.F3 == nil {
		return nil
	}
	return m.F3.Stop(ctx)
}
