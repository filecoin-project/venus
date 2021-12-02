package v1

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/wallet"
)

type IWallet interface {
	// Rule[perm:sign]
	WalletSign(ctx context.Context, k address.Address, msg []byte, meta wallet.MsgMeta) (*crypto.Signature, error)
	// Rule[perm:admin]
	WalletExport(addr address.Address, password string) (*crypto.KeyInfo, error)
	// Rule[perm:admin]
	WalletImport(key *crypto.KeyInfo) (address.Address, error)
	// Rule[perm:write]
	WalletHas(ctx context.Context, addr address.Address) (bool, error)
	// Rule[perm:write]
	WalletNewAddress(protocol address.Protocol) (address.Address, error)
	// Rule[perm:read]
	WalletBalance(ctx context.Context, addr address.Address) (abi.TokenAmount, error) //not exists in remote
	// Rule[perm:write]
	WalletDefaultAddress(ctx context.Context) (address.Address, error) //not exists in remote
	// Rule[perm:admin]
	WalletAddresses(ctx context.Context) []address.Address
	// Rule[perm:admin]
	WalletSetDefault(ctx context.Context, addr address.Address) error //not exists in remote
	// Rule[perm:sign]
	WalletSignMessage(ctx context.Context, k address.Address, msg *types.UnsignedMessage) (*types.SignedMessage, error)
	// Rule[perm:admin]
	LockWallet(ctx context.Context) error
	// Rule[perm:admin]
	UnLockWallet(ctx context.Context, password []byte) error
	// Rule[perm:admin]
	SetPassword(Context context.Context, password []byte) error
	// Rule[perm:admin]
	HasPassword(Context context.Context) bool
	// Rule[perm:admin]
	WalletState(Context context.Context) int
}
