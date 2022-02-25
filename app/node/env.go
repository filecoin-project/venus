package node

import (
	"context"

	cmds "github.com/ipfs/go-ipfs-cmds"

	"github.com/filecoin-project/venus/app/submodule/storagenetworking"
	v0api "github.com/filecoin-project/venus/venus-shared/api/chain/v0"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
)

// Env is the environment for command API handlers.
type Env struct {
	ctx                  context.Context
	InspectorAPI         IInspector
	BlockStoreAPI        v1api.IBlockStore
	ChainAPI             v1api.IChain
	NetworkAPI           v1api.INetwork
	StorageNetworkingAPI storagenetworking.IStorageNetworking
	SyncerAPI            v1api.ISyncer
	WalletAPI            v1api.IWallet
	MingingAPI           v1api.IMining
	MessagePoolAPI       v1api.IMessagePool

	MultiSigAPI v0api.IMultiSig
	MarketAPI   v1api.IMarket
	PaychAPI    v1api.IPaychan
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
