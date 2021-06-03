package mining

import (
	"github.com/filecoin-project/venus/app/submodule/apiface"
	"github.com/filecoin-project/venus/app/submodule/blockstore"
	chain2 "github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/app/submodule/network"
	"github.com/filecoin-project/venus/app/submodule/syncer"
	"github.com/filecoin-project/venus/app/submodule/wallet"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/util/ffiwrapper"
)

type miningConfig interface {
	Repo() repo.Repo
}

type MiningModule struct { //nolint
	Config        miningConfig
	ChainModule   *chain2.ChainSubmodule
	BlockStore    *blockstore.BlockstoreSubmodule
	NetworkModule *network.NetworkSubmodule
	SyncModule    *syncer.SyncerSubmodule
	Wallet        wallet.WalletSubmodule
	proofVerifier ffiwrapper.Verifier
}

func (miningModule *MiningModule) API() apiface.IMining {
	return &MiningAPI{Ming: miningModule}
}

func (miningModule *MiningModule) V0API() apiface.IMining {
	return &MiningAPI{Ming: miningModule}
}

func NewMiningModule(
	conf miningConfig,
	chainModule *chain2.ChainSubmodule,
	blockStore *blockstore.BlockstoreSubmodule,
	networkModule *network.NetworkSubmodule,
	syncModule *syncer.SyncerSubmodule,
	wallet wallet.WalletSubmodule,
	proofVerifier ffiwrapper.Verifier,
) *MiningModule {
	return &MiningModule{
		Config:        conf,
		ChainModule:   chainModule,
		BlockStore:    blockStore,
		NetworkModule: networkModule,
		SyncModule:    syncModule,
		Wallet:        wallet,
		proofVerifier: proofVerifier,
	}
}
