package mining

import (
	"github.com/filecoin-project/venus/app/submodule/blockstore"
	chain2 "github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/app/submodule/network"
	"github.com/filecoin-project/venus/app/submodule/syncer"
	"github.com/filecoin-project/venus/app/submodule/wallet"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/statemanger"
	"github.com/filecoin-project/venus/pkg/util/ffiwrapper"
	v0api "github.com/filecoin-project/venus/venus-shared/api/chain/v0"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
)

type miningConfig interface {
	Repo() repo.Repo
	Verifier() ffiwrapper.Verifier
}

// MiningModule enhances the `Node` with miner capabilities.
type MiningModule struct { //nolint
	Config        miningConfig
	ChainModule   *chain2.ChainSubmodule
	BlockStore    *blockstore.BlockstoreSubmodule
	NetworkModule *network.NetworkSubmodule
	SyncModule    *syncer.SyncerSubmodule
	Wallet        wallet.WalletSubmodule
	proofVerifier ffiwrapper.Verifier
	Stmgr         *statemanger.Stmgr
}

//API create new miningAPi implement
func (miningModule *MiningModule) API() v1api.IMining {
	return &MiningAPI{Ming: miningModule}
}

func (miningModule *MiningModule) V0API() v0api.IMining {
	return &MiningAPI{Ming: miningModule}
}

//NewMiningModule create new mining module
func NewMiningModule(
	stmgr *statemanger.Stmgr,
	conf miningConfig,
	chainModule *chain2.ChainSubmodule,
	blockStore *blockstore.BlockstoreSubmodule,
	networkModule *network.NetworkSubmodule,
	syncModule *syncer.SyncerSubmodule,
	wallet wallet.WalletSubmodule,
) *MiningModule {
	return &MiningModule{
		Stmgr:         stmgr,
		Config:        conf,
		ChainModule:   chainModule,
		BlockStore:    blockStore,
		NetworkModule: networkModule,
		SyncModule:    syncModule,
		Wallet:        wallet,
		proofVerifier: conf.Verifier(),
	}
}
