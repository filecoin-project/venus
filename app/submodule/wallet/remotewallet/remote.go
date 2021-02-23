package remotewallet

import (
	"context"
	"github.com/ipfs-force-community/venus-wallet/api"
	"github.com/ipfs-force-community/venus-wallet/api/remotecli"
	"golang.org/x/xerrors"
)

type RemoteWallet struct {
	api.IWallet
	Cancel func()
}

func SetupRemoteWallet(info string) (*RemoteWallet, error) {
	ai, err := remotecli.ParseApiInfo(info)
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

func (w *RemoteWallet) Get() api.IWallet {
	if w == nil {
		return nil
	}
	return w
}
