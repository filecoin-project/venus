package mpool

import (
	"github.com/filecoin-project/venus/app/submodule/syncer"
	logging "github.com/ipfs/go-log"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/app/submodule/network"
	"github.com/filecoin-project/venus/app/submodule/wallet"
	"github.com/filecoin-project/venus/pkg/messagepool"
	"github.com/filecoin-project/venus/pkg/messagepool/journal"
	"github.com/filecoin-project/venus/pkg/repo"
)

var log = logging.Logger("mpool")

type messagepoolConfig interface {
	Repo() repo.Repo
}

// MessagingSubmodule enhances the `Node` with internal messaging capabilities.
type MessagePoolSubmodule struct { //nolint
	MPool *messagepool.MessagePool
}

func OpenFilesystemJournal(lr repo.Repo) (journal.Journal, error) {
	jrnl, err := journal.OpenFSJournal(lr, journal.EnvDisabledEvents())
	if err != nil {
		return nil, err
	}

	return jrnl, err
}

func NewMpoolSubmodule(cfg messagepoolConfig, network *network.NetworkSubmodule, chain *chain.ChainSubmodule, syncer *syncer.SyncerSubmodule) (*MessagePoolSubmodule, error) {
	mpp := messagepool.NewProvider(chain.ChainReader, chain.MessageStore, cfg.Repo().Config().NetworkParams, network.Pubsub)

	j, err := OpenFilesystemJournal(cfg.Repo())
	if err != nil {
		return nil, err
	}
	mp, err := messagepool.New(mpp, cfg.Repo().MetaDatastore(), network.NetworkName, syncer.Consensus, chain.State, j)
	if err != nil {
		return nil, xerrors.Errorf("constructing mpool: %s", err)
	}

	return &MessagePoolSubmodule{MPool: mp}, nil
}

func (mp *MessagePoolSubmodule) Close() {
	err := mp.MPool.Close()
	if err != nil {
		log.Errorf("failed to close mpool: %s", err)
	}
}

func (mp *MessagePoolSubmodule) API(walletAPI *wallet.WalletAPI, chainAPI *chain.ChainAPI) *MessagePoolAPI {
	return &MessagePoolAPI{walletAPI: walletAPI, chainAPI: chainAPI, mp: mp}
}
