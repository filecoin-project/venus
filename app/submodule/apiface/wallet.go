package apiface

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/wallet"
)

type IWallet interface {
	// Rule[perm:admin,ignore:true]
	WalletSign(ctx context.Context, k address.Address, msg []byte, meta wallet.MsgMeta) (*crypto.Signature, error)
	// Rule[perm:admin,ignore:true]
	WalletExport(addr address.Address, password string) (*crypto.KeyInfo, error)
	// Rule[perm:admin,ignore:true]
	WalletImport(key *crypto.KeyInfo) (address.Address, error)
	// Rule[perm:admin,ignore:true]
	WalletHas(ctx context.Context, addr address.Address) (bool, error)
	// Rule[perm:admin,ignore:true]
	WalletNewAddress(protocol address.Protocol) (address.Address, error)
	// Rule[perm:admin,ignore:true]
	WalletBalance(ctx context.Context, addr address.Address) (abi.TokenAmount, error) //not exists in remote
	// Rule[perm:admin,ignore:true]
	WalletDefaultAddress(ctx context.Context) (address.Address, error) //not exists in remote
	// Rule[perm:admin,ignore:true]
	WalletAddresses(ctx context.Context) []address.Address
	// Rule[perm:admin,ignore:true]
	WalletSetDefault(ctx context.Context, addr address.Address) error //not exists in remote
	// Rule[perm:admin,ignore:true]
	WalletSignMessage(ctx context.Context, k address.Address, msg *types.UnsignedMessage) (*types.SignedMessage, error)
	// Rule[perm:admin,ignore:true]
	LockWallet(ctx context.Context) error
	// Rule[perm:admin,ignore:true]
	UnLockWallet(ctx context.Context, password string) error
	// Rule[perm:admin,ignore:true]
	SetPassword(Context context.Context, password string) error
	// Rule[perm:admin,ignore:true]
	HasPassword(Context context.Context) bool
	// Rule[perm:admin,ignore:true]
	WalletState(Context context.Context) int
}
