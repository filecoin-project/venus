package remotewallet

import (
	"context"
	"fmt"

	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/filecoin-project/go-address"

	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/wallet"
)

var _ wallet.WalletIntersection = &remoteWallet{}

type remoteWallet struct {
	IWallet
	Cancel func()
}

func (w *remoteWallet) Addresses(ctx context.Context) []address.Address {
	wallets, err := w.IWallet.WalletList(context.Background())
	if err != nil {
		return make([]address.Address, 0)
	}
	return wallets
}

func (w *remoteWallet) HasPassword(ctx context.Context) bool {
	return true
}

func SetupRemoteWallet(info string) (wallet.WalletIntersection, error) {
	ai, err := ParseAPIInfo(info)
	if err != nil {
		return nil, err
	}
	url, err := ai.DialArgs()
	if err != nil {
		return nil, err
	}
	wapi, closer, err := NewWalletRPC(context.Background(), url, ai.AuthHeader())
	if err != nil {
		return nil, fmt.Errorf("creating jsonrpc client: %w", err)
	}
	return &remoteWallet{
		IWallet: wapi,
		Cancel:  closer,
	}, nil
}

func (w *remoteWallet) HasAddress(ctx context.Context, addr address.Address) bool {
	exist, err := w.IWallet.WalletHas(context.Background(), addr)
	if err != nil {
		return false
	}
	return exist
}
func (w *remoteWallet) NewAddress(ctx context.Context, protocol address.Protocol) (address.Address, error) {
	return w.IWallet.WalletNew(context.Background(), GetKeyType(protocol))
}

func (w *remoteWallet) Import(ctx context.Context, key *crypto.KeyInfo) (address.Address, error) {
	return w.IWallet.WalletImport(context.Background(), ConvertRemoteKeyInfo(key))
}

func (w *remoteWallet) Export(ctx context.Context, addr address.Address, password string) (*crypto.KeyInfo, error) {
	key, err := w.IWallet.WalletExport(context.Background(), addr)
	if err != nil {
		return nil, err
	}
	return ConvertLocalKeyInfo(key), nil
}

func (w *remoteWallet) WalletSign(ctx context.Context, keyAddr address.Address, msg []byte, meta types.MsgMeta) (*crypto.Signature, error) {
	return w.IWallet.WalletSign(context.Background(), keyAddr, msg, meta)
}
