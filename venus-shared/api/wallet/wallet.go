package wallet

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/venus/venus-shared/types"
)

type ILocalWallet interface {
	IWallet
	IWalletLock
}

type IWalletLock interface {
	// SetPassword do it first after program setup
	SetPassword(ctx context.Context, password string) error //perm:admin
	// unlock the wallet and enable IWallet logic
	Unlock(ctx context.Context, password string) error //perm:admin
	// lock the wallet and disable IWallet logic
	Lock(ctx context.Context, password string) error //perm:admin
	// show lock state
	LockState(ctx context.Context) bool //perm:admin
	// VerifyPassword verify that the passwords are consistent
	VerifyPassword(ctx context.Context, password string) error //perm:admin
}

type IWallet interface {
	WalletNew(ctx context.Context, kt types.KeyType) (address.Address, error)                                             //perm:admin
	WalletHas(ctx context.Context, address address.Address) (bool, error)                                                 //perm:read
	WalletList(ctx context.Context) ([]address.Address, error)                                                            //perm:read
	WalletSign(ctx context.Context, signer address.Address, toSign []byte, meta types.MsgMeta) (*crypto.Signature, error) //perm:sign
	WalletExport(ctx context.Context, addr address.Address) (*types.KeyInfo, error)                                       //perm:admin
	WalletImport(ctx context.Context, ki *types.KeyInfo) (address.Address, error)                                         //perm:admin
	WalletDelete(ctx context.Context, addr address.Address) error                                                         //perm:admin
}

type IWalletEvent interface {
	AddSupportAccount(ctx context.Context, supportAccount string) error  //perm:admin
	AddNewAddress(ctx context.Context, newAddrs []address.Address) error //perm:admin
}
