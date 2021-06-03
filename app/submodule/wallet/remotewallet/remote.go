package remotewallet

import (
	"context"
	"net/http"

	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"

	"github.com/filecoin-project/go-jsonrpc"

	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/wallet"
)

type IWallet interface {
	WalletNew(context.Context, wallet.KeyType) (address.Address, error)
	WalletHas(ctx context.Context, address address.Address) (bool, error)
	WalletList(ctx context.Context) ([]address.Address, error)
	WalletSign(ctx context.Context, signer address.Address, toSign []byte, meta wallet.MsgMeta) (*crypto.Signature, error)
	WalletExport(ctx context.Context, addr address.Address) (*wallet.KeyInfo, error)
	WalletImport(context.Context, *wallet.KeyInfo) (address.Address, error)
	WalletDelete(context.Context, address.Address) error
}

var _ IWallet = &WalletAPIAdapter{}

// wallet API permissions constraints
type WalletAPIAdapter struct {
	Internal struct {
		WalletNew    func(ctx context.Context, kt wallet.KeyType) (address.Address, error)                                          `perm:"admin"`
		WalletHas    func(ctx context.Context, address address.Address) (bool, error)                                             `perm:"write"`
		WalletList   func(ctx context.Context) ([]address.Address, error)                                                         `perm:"write"`
		WalletSign   func(ctx context.Context, signer address.Address, toSign []byte, meta wallet.MsgMeta) (*crypto.Signature, error) `perm:"sign"`
		WalletExport func(ctx context.Context, addr address.Address) (*wallet.KeyInfo, error)                                       `perm:"admin"`
		WalletImport func(ctx context.Context, ki *wallet.KeyInfo) (address.Address, error)                                         `perm:"admin"`
		WalletDelete func(ctx context.Context, addr address.Address) error                                                        `perm:"admin"`
	}
}

func (c *WalletAPIAdapter) WalletNew(ctx context.Context, keyType wallet.KeyType) (address.Address, error) {
	return c.Internal.WalletNew(ctx, keyType)
}

func (c *WalletAPIAdapter) WalletHas(ctx context.Context, addr address.Address) (bool, error) {
	return c.Internal.WalletHas(ctx, addr)
}

func (c *WalletAPIAdapter) WalletList(ctx context.Context) ([]address.Address, error) {
	return c.Internal.WalletList(ctx)
}

func (c *WalletAPIAdapter) WalletSign(ctx context.Context, signer address.Address, toSign []byte, meta wallet.MsgMeta) (*crypto.Signature, error) {
	return c.Internal.WalletSign(ctx, signer, toSign, meta)
}

func (c *WalletAPIAdapter) WalletExport(ctx context.Context, a address.Address) (*wallet.KeyInfo, error) {
	return c.Internal.WalletExport(ctx, a)
}

func (c *WalletAPIAdapter) WalletImport(ctx context.Context, ki *wallet.KeyInfo) (address.Address, error) {
	return c.Internal.WalletImport(ctx, ki)
}

func (c *WalletAPIAdapter) WalletDelete(ctx context.Context, addr address.Address) error {
	return c.Internal.WalletDelete(ctx, addr)
}

// NewWalletRPC RPCClient returns an RPC client connected to a node
// @addr			reference ./httpparse/ParseApiInfo()
// @requestHeader 	reference ./httpparse/ParseApiInfo()
func NewWalletRPC(ctx context.Context, addr string, requestHeader http.Header) (IWallet, jsonrpc.ClientCloser, error) {
	var res WalletAPIAdapter
	closer, err := jsonrpc.NewMergeClient(ctx, addr, "Filecoin",
		[]interface{}{
			&res.Internal,
		},
		requestHeader,
	)
	return &res, closer, err
}

var _ wallet.WalletIntersection = &remoteWallet{}

type remoteWallet struct {
	IWallet
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

func SetupRemoteWallet(info string) (wallet.WalletIntersection, error) {
	ai, err := wallet.ParseApiInfo(info)
	if err != nil {
		return nil, err
	}
	url, err := ai.DialArgs()
	if err != nil {
		return nil, err
	}
	wapi, closer, err := NewWalletRPC(context.Background(), url, ai.AuthHeader())
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

func (w *remoteWallet) WalletSign(keyAddr address.Address, msg []byte, meta wallet.MsgMeta) (*crypto.Signature, error) {
	return w.IWallet.WalletSign(context.Background(), keyAddr, msg, meta)
}
