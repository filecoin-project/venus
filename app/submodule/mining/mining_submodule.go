package mining

import (
	"github.com/filecoin-project/venus/app/submodule/blockstore"
	chain2 "github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/app/submodule/network"
	"github.com/filecoin-project/venus/app/submodule/proofverification"
	"github.com/filecoin-project/venus/app/submodule/syncer"
	"github.com/filecoin-project/venus/app/submodule/wallet"
	"github.com/filecoin-project/venus/pkg/repo"
)

type miningConfig interface {
	Repo() repo.Repo
}

type MiningModule struct {
	Config            miningConfig
	ChainModule       *chain2.ChainSubmodule
	BlockStore        *blockstore.BlockstoreSubmodule
	NetworkModule     *network.NetworkSubmodule
	SyncModule        *syncer.SyncerSubmodule
	Wallet            wallet.WalletSubmodule
	ProofVerification proofverification.ProofVerificationSubmodule
}

func (miningModule *MiningModule) API() *MiningAPI {
	return &MiningAPI{Ming: miningModule}
}

func NewMiningModule(
	conf miningConfig,
	chainModule *chain2.ChainSubmodule,
	blockStore *blockstore.BlockstoreSubmodule,
	networkModule *network.NetworkSubmodule,
	syncModule *syncer.SyncerSubmodule,
	wallet wallet.WalletSubmodule,
	proofVerification proofverification.ProofVerificationSubmodule,
) *MiningModule {
	return &MiningModule{
		Config:            conf,
		ChainModule:       chainModule,
		BlockStore:        blockStore,
		NetworkModule:     networkModule,
		SyncModule:        syncModule,
		Wallet:            wallet,
		ProofVerification: proofVerification,
	}
}
