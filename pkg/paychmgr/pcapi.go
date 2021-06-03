package paychmgr

import (
	"context"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"

	"github.com/filecoin-project/venus/app/submodule/apiface"
	"github.com/filecoin-project/venus/app/submodule/apitypes"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/wallet"
)

// paychAPI defines the API methods needed by the payment channel manager
type paychAPI interface {
	StateAccountKey(context.Context, address.Address, types.TipSetKey) (address.Address, error)
	StateWaitMsg(ctx context.Context, msg cid.Cid, confidence abi.ChainEpoch) (*apitypes.MsgLookup, error)
	WalletHas(ctx context.Context, addr address.Address) (bool, error)
	WalletSign(ctx context.Context, k address.Address, msg []byte) (*crypto.Signature, error)
	StateNetworkVersion(context.Context, types.TipSetKey) (network.Version, error)
	MpoolPushMessage(ctx context.Context, msg *types.UnsignedMessage, maxFee *types.MessageSendSpec) (*types.SignedMessage, error)
}

type pcAPI struct {
	mpAPI        apiface.IMessagePool
	chainInfoAPI apiface.IChainInfo
	accountAPI   apiface.IAccount
	walletAPI    apiface.IWallet
}

func newPaychAPI(mpAPI apiface.IMessagePool, c apiface.IChain, w apiface.IWallet) paychAPI {
	return &pcAPI{mpAPI: mpAPI, chainInfoAPI: c, accountAPI: c, walletAPI: w}
}

func (o *pcAPI) StateAccountKey(ctx context.Context, address address.Address, tsk types.TipSetKey) (address.Address, error) {
	return o.accountAPI.StateAccountKey(ctx, address, tsk)
}
func (o *pcAPI) StateWaitMsg(ctx context.Context, msg cid.Cid, confidence abi.ChainEpoch) (*apitypes.MsgLookup, error) {
	return o.chainInfoAPI.StateWaitMsg(ctx, msg, confidence)
}
func (o *pcAPI) MpoolPushMessage(ctx context.Context, msg *types.UnsignedMessage, maxFee *types.MessageSendSpec) (*types.SignedMessage, error) {
	return o.mpAPI.MpoolPushMessage(ctx, msg, maxFee)
}
func (o *pcAPI) WalletHas(ctx context.Context, addr address.Address) (bool, error) {
	return o.walletAPI.WalletHas(ctx, addr)
}
func (o *pcAPI) WalletSign(ctx context.Context, k address.Address, msg []byte) (*crypto.Signature, error) {
	return o.walletAPI.WalletSign(ctx, k, msg, wallet.MsgMeta{Type: wallet.MTSignedVoucher})
}
func (o *pcAPI) StateNetworkVersion(ctx context.Context, ts types.TipSetKey) (network.Version, error) {
	return o.chainInfoAPI.StateNetworkVersion(ctx, ts)
}
