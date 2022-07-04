package wallet

import (
	"context"
	"errors"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/venus/app/submodule/wallet/remotewallet"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/wallet"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var _ v1api.IWallet = &WalletAPI{}

var ErrNoDefaultFromAddress = errors.New("unable to determine a default walletModule address")

type WalletAPI struct { // nolint
	walletModule *WalletSubmodule
	adapter      wallet.WalletIntersection
}

// WalletBalance returns the current balance of the given wallet address.
func (walletAPI *WalletAPI) WalletBalance(ctx context.Context, addr address.Address) (abi.TokenAmount, error) {
	actor, err := walletAPI.walletModule.Chain.Stmgr.GetActorAtTsk(ctx, addr, types.EmptyTSK)
	if err != nil {
		if errors.Is(err, types.ErrActorNotFound) {
			return abi.NewTokenAmount(0), nil
		}
		return abi.NewTokenAmount(0), err
	}

	return actor.Balance, nil
}

// WalletHas indicates whether the given address is in the wallet.
func (walletAPI *WalletAPI) WalletHas(ctx context.Context, addr address.Address) (bool, error) {
	return walletAPI.adapter.HasAddress(ctx, addr), nil
}

// SetWalletDefaultAddress set the specified address as the default in the config.
func (walletAPI *WalletAPI) WalletDefaultAddress(ctx context.Context) (address.Address, error) {
	ret, err := walletAPI.walletModule.Config.Get("walletModule.defaultAddress")
	addr := ret.(address.Address)
	if err != nil || !addr.Empty() {
		return addr, err
	}

	// No default is set; pick the 0th and make it the default.
	if len(walletAPI.WalletAddresses(ctx)) > 0 {
		addr := walletAPI.WalletAddresses(ctx)[0]
		err := walletAPI.walletModule.Config.Set("walletModule.defaultAddress", addr.String())
		if err != nil {
			return address.Undef, err
		}

		return addr, nil
	}

	return address.Undef, nil
}

// WalletAddresses gets addresses from the walletModule
func (walletAPI *WalletAPI) WalletAddresses(ctx context.Context) []address.Address {
	return walletAPI.adapter.Addresses(ctx)
}

// SetWalletDefaultAddress set the specified address as the default in the config.
func (walletAPI *WalletAPI) WalletSetDefault(ctx context.Context, addr address.Address) error {
	localAddrs := walletAPI.WalletAddresses(ctx)
	for _, localAddr := range localAddrs {
		if localAddr == addr {
			err := walletAPI.walletModule.Config.Set("walletModule.defaultAddress", addr.String())
			if err != nil {
				return err
			}
			return nil
		}
	}
	return errors.New("addr not in the walletModule list")
}

// WalletNewAddress generates a new walletModule address
func (walletAPI *WalletAPI) WalletNewAddress(ctx context.Context, protocol address.Protocol) (address.Address, error) {
	return walletAPI.adapter.NewAddress(ctx, protocol)
}

// WalletImport adds a given set of KeyInfos to the walletModule
func (walletAPI *WalletAPI) WalletImport(ctx context.Context, key *types.KeyInfo) (address.Address, error) {
	addr, err := walletAPI.adapter.Import(ctx, remotewallet.ConvertLocalKeyInfo(key))
	if err != nil {
		return address.Undef, err
	}
	return addr, nil
}

// WalletExport returns the KeyInfos for the given walletModule addresses
func (walletAPI *WalletAPI) WalletExport(ctx context.Context, addr address.Address, password string) (*types.KeyInfo, error) {
	ki, err := walletAPI.adapter.Export(ctx, addr, password)
	if err != nil {
		return nil, err
	}
	return remotewallet.ConvertRemoteKeyInfo(ki), nil
}

// WalletSign signs the given bytes using the given address.
func (walletAPI *WalletAPI) WalletSign(ctx context.Context, k address.Address, msg []byte, meta types.MsgMeta) (*crypto.Signature, error) {
	keyAddr, err := walletAPI.walletModule.Chain.Stmgr.ResolveToKeyAddress(ctx, k, nil)
	if err != nil {
		return nil, fmt.Errorf("ResolveTokeyAddress failed:%v", err)
	}
	return walletAPI.adapter.WalletSign(ctx, keyAddr, msg, meta)
}

// WalletSignMessage signs the given message using the given address.
func (walletAPI *WalletAPI) WalletSignMessage(ctx context.Context, k address.Address, msg *types.Message) (*types.SignedMessage, error) {
	mb, err := msg.ToStorageBlock()
	if err != nil {
		return nil, fmt.Errorf("serializing message: %w", err)
	}

	sign, err := walletAPI.WalletSign(ctx, k, mb.Cid().Bytes(), types.MsgMeta{Type: types.MTChainMsg})
	if err != nil {
		return nil, fmt.Errorf("failed to sign message: %w", err)
	}

	return &types.SignedMessage{
		Message:   *msg,
		Signature: *sign,
	}, nil
}

// LockWallet lock wallet
func (walletAPI *WalletAPI) LockWallet(ctx context.Context) error {
	return walletAPI.walletModule.Wallet.LockWallet(ctx)
}

// UnLockWallet unlock wallet
func (walletAPI *WalletAPI) UnLockWallet(ctx context.Context, password []byte) error {
	return walletAPI.walletModule.Wallet.UnLockWallet(ctx, password)
}

// SetPassword set wallet password
func (walletAPI *WalletAPI) SetPassword(ctx context.Context, password []byte) error {
	return walletAPI.walletModule.Wallet.SetPassword(ctx, password)
}

// HasPassword return whether the wallet has password
func (walletAPI *WalletAPI) HasPassword(ctx context.Context) bool {
	return walletAPI.adapter.HasPassword(ctx)
}

// WalletState return wallet state
func (walletAPI *WalletAPI) WalletState(ctx context.Context) int {
	return walletAPI.walletModule.Wallet.WalletState(ctx)
}
