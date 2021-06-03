// Code from github.com/filecoin-project/venus-wallet/storage/wallet/wallet.go & api/api_wallet.go & api/remotecli/cli.go . DO NOT EDIT.

package remotewallet

import (
	"context"
	"net/http"

	"github.com/filecoin-project/go-jsonrpc"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/pkg/wallet"
)

type IWallet interface {
	WalletNew(context.Context, wallet.KeyType) (address.Address, error)
	WalletHas(ctx context.Context, address address.Address) (bool, error)
	WalletList(ctx context.Context) ([]address.Address, error)
	WalletSign(ctx context.Context, signer address.Address, toSign []byte, meta wallet.MsgMeta) (*wallet.Signature, error)
	WalletExport(ctx context.Context, addr address.Address) (*wallet.KeyInfo, error)
	WalletImport(context.Context, *wallet.KeyInfo) (address.Address, error)
	WalletDelete(context.Context, address.Address) error
}

var _ IWallet = &WalletAPIAdapter{}

// wallet API permissions constraints
type WalletAPIAdapter struct {
	Internal struct {
		WalletNew    func(ctx context.Context, kt wallet.KeyType) (address.Address, error)                                            `perm:"admin"`
		WalletHas    func(ctx context.Context, address address.Address) (bool, error)                                                 `perm:"write"`
		WalletList   func(ctx context.Context) ([]address.Address, error)                                                             `perm:"write"`
		WalletSign   func(ctx context.Context, signer address.Address, toSign []byte, meta wallet.MsgMeta) (*wallet.Signature, error) `perm:"sign"`
		WalletExport func(ctx context.Context, addr address.Address) (*wallet.KeyInfo, error)                                         `perm:"admin"`
		WalletImport func(ctx context.Context, ki *wallet.KeyInfo) (address.Address, error)                                           `perm:"admin"`
		WalletDelete func(ctx context.Context, addr address.Address) error                                                            `perm:"admin"`
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

func (c *WalletAPIAdapter) WalletSign(ctx context.Context, signer address.Address, toSign []byte, meta wallet.MsgMeta) (*wallet.Signature, error) {
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
