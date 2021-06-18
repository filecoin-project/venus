package node

import (
	"context"

	"github.com/filecoin-project/venus/app/submodule/apiface"
	"github.com/filecoin-project/venus/app/submodule/apiface/v0api"
	cmds "github.com/ipfs/go-ipfs-cmds"

	"github.com/filecoin-project/venus/app/submodule/storagenetworking"
)

// Env is the environment for command API handlers.
type Env struct {
	ctx                  context.Context
	InspectorAPI         IInspector
	BlockServiceAPI      apiface.IBlockService
	BlockStoreAPI        apiface.IBlockStore
	ChainAPI             apiface.IChain
	ConfigAPI            apiface.IConfig
	DiscoveryAPI         apiface.IDiscovery
	NetworkAPI           apiface.INetwork
	StorageNetworkingAPI storagenetworking.IStorageNetworking
	SyncerAPI            apiface.ISyncer
	WalletAPI            apiface.IWallet
	MingingAPI           apiface.IMining
	MessagePoolAPI       apiface.IMessagePool

	MultiSigAPI v0api.IMultiSig
	MarketAPI   apiface.IMarket
	PaychAPI    apiface.IPaychan
}

var _ cmds.Environment = (*Env)(nil)

// NewClientEnv returns a new environment for command API clients.
// This environment lacks direct access to any internal APIs.
func NewClientEnv(ctx context.Context) *Env {
	return &Env{ctx: ctx}
}

// Context returns the context of the environment.
func (ce *Env) Context() context.Context {
	return ce.ctx
}
