package remotewallet

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus-wallet/api/remotecli"
	"github.com/filecoin-project/venus-wallet/api/remotecli/httpparse"
	"github.com/filecoin-project/venus-wallet/storage/wallet"
	"github.com/filecoin-project/venus/pkg/crypto"
	locWallet "github.com/filecoin-project/venus/pkg/wallet"
	"golang.org/x/xerrors"
)

var _ locWallet.WalletIntersection = &remoteWallet{}

type remoteWallet struct {
	wallet.IWallet
	Cancel func()
}

func (w *remoteWallet) Addresses() []address.Address {
	wallets, err := w.IWallet.WalletList(context.Background())
	if err != nil {
		return make([]address.Address, 0)
	}
	return wallets
}

func (w *remoteWallet) HasPassword() bool {
	return true
}

func SetupRemoteWallet(info string) (locWallet.WalletIntersection, error) {
	ai, err := httpparse.ParseApiInfo(info)
	if err != nil {
		return nil, err
	}
	url, err := ai.DialArgs()
	if err != nil {
		return nil, err
	}
	wapi, closer, err := remotecli.NewWalletRPC(context.Background(), url, ai.AuthHeader())
	if err != nil {
		return nil, xerrors.Errorf("creating jsonrpc client: %w", err)
	}
	return &remoteWallet{
		IWallet: wapi,
		Cancel:  closer,
	}, nil
}

func (w *remoteWallet) HasAddress(addr address.Address) bool {
	exist, err := w.IWallet.WalletHas(context.Background(), addr)
	if err != nil {
		return false
	}
	return exist
}
func (w *remoteWallet) NewAddress(protocol address.Protocol) (address.Address, error) {
	return w.IWallet.WalletNew(context.Background(), GetKeyType(protocol))
}

func (w *remoteWallet) Import(key *crypto.KeyInfo) (address.Address, error) {
	return w.IWallet.WalletImport(context.Background(), ConvertRemoteKeyInfo(key))
}

func (w *remoteWallet) Export(addr address.Address, password string) (*crypto.KeyInfo, error) {
	key, err := w.IWallet.WalletExport(context.Background(), addr)
	if err != nil {
		return nil, err
	}
	return ConvertLocalKeyInfo(key), nil
}

func (w *remoteWallet) WalletSign(keyAddr address.Address, msg []byte, meta locWallet.MsgMeta) (*crypto.Signature, error) {
	return w.IWallet.WalletSign(context.Background(), keyAddr, msg, meta)
}
