package remotewallet

import (
	"context"
	"github.com/ipfs-force-community/venus-wallet/api/remotecli"
	"github.com/ipfs-force-community/venus-wallet/api/remotecli/httpparse"
	"github.com/ipfs-force-community/venus-wallet/storage/wallet"
	"golang.org/x/xerrors"
)

type RemoteWallet struct {
	wallet.IWallet
	Cancel func()
}

func SetupRemoteWallet(info string) (*RemoteWallet, error) {
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
	return &RemoteWallet{
		IWallet: wapi,
		Cancel:  closer,
	}, nil
}

func (w *RemoteWallet) Get() wallet.IWallet {
	if w == nil {
		return nil
	}
	return w
}
