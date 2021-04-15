package wallet

import (
	"context"
	"errors"
	"strings"

	"github.com/ipfs-force-community/venus-wallet/core"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/wallet"
)

var _ IWallet = &WalletAPI{}

type IWallet interface {
	WalletSign(ctx context.Context, k address.Address, msg []byte, meta wallet.MsgMeta) (*crypto.Signature, error)
	WalletExport(addr address.Address, password string) (*crypto.KeyInfo, error)
	WalletImport(key *crypto.KeyInfo) (address.Address, error)
	WalletHas(ctx context.Context, addr address.Address) (bool, error)
	WalletNewAddress(protocol address.Protocol) (address.Address, error)
	WalletBalance(ctx context.Context, addr address.Address) (abi.TokenAmount, error) //not exists in remote
	WalletDefaultAddress() (address.Address, error)                                   //not exists in remote
	WalletAddresses() []address.Address
	WalletSetDefault(_ context.Context, addr address.Address) error //not exists in remote
	WalletSignMessage(ctx context.Context, k address.Address, msg *types.UnsignedMessage) (*types.SignedMessage, error)
}

var ErrNoDefaultFromAddress = errors.New("unable to determine a default walletModule address")

type WalletAPI struct { //nolint
	walletModule *WalletSubmodule
	adapter      wallet.WalletIntersection
}

// WalletBalance returns the current balance of the given wallet address.
func (walletAPI *WalletAPI) WalletBalance(ctx context.Context, addr address.Address) (abi.TokenAmount, error) {
	headkey := walletAPI.walletModule.Chain.ChainReader.GetHead()
	act, err := walletAPI.walletModule.Chain.ChainReader.GetActorAt(ctx, headkey, addr)
	if err != nil && strings.Contains(err.Error(), types.ErrActorNotFound.Error()) {
		return abi.NewTokenAmount(0), nil
	} else if err != nil {
		return abi.NewTokenAmount(0), err
	}

	return act.Balance, nil
}

func (walletAPI *WalletAPI) WalletHas(ctx context.Context, addr address.Address) (bool, error) {
	return walletAPI.adapter.HasAddress(ctx, addr), nil
}

// SetWalletDefaultAddress set the specified address as the default in the config.
func (walletAPI *WalletAPI) WalletDefaultAddress() (address.Address, error) {
	ret, err := walletAPI.walletModule.Config.Get("walletModule.defaultAddress")
	addr := ret.(address.Address)
	if err != nil || !addr.Empty() {
		return addr, err
	}

	// No default is set; pick the 0th and make it the default.
	if len(walletAPI.WalletAddresses()) > 0 {
		addr := walletAPI.WalletAddresses()[0]
		err := walletAPI.walletModule.Config.Set("walletModule.defaultAddress", addr.String())
		if err != nil {
			return address.Undef, err
		}

		return addr, nil
	}

	return address.Undef, nil
}

// WalletAddresses gets addresses from the walletModule
func (walletAPI *WalletAPI) WalletAddresses() []address.Address {
	return walletAPI.adapter.Addresses()
}

// SetWalletDefaultAddress set the specified address as the default in the config.
func (walletAPI *WalletAPI) WalletSetDefault(_ context.Context, addr address.Address) error {
	localAddrs := walletAPI.WalletAddresses()
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
func (walletAPI *WalletAPI) WalletNewAddress(protocol address.Protocol) (address.Address, error) {
	return walletAPI.adapter.NewAddress(protocol)
}

// WalletImport adds a given set of KeyInfos to the walletModule
func (walletAPI *WalletAPI) WalletImport(key *crypto.KeyInfo) (address.Address, error) {
	addr, err := walletAPI.adapter.Import(key)
	if err != nil {
		return address.Undef, err
	}
	return addr, nil
}

// WalletExport returns the KeyInfos for the given walletModule addresses
func (walletAPI *WalletAPI) WalletExport(addr address.Address, password string) (*crypto.KeyInfo, error) {
	return walletAPI.adapter.Export(addr, password)
}

func (walletAPI *WalletAPI) WalletSign(ctx context.Context, k address.Address, msg []byte, meta wallet.MsgMeta) (*crypto.Signature, error) {
	head := walletAPI.walletModule.Chain.ChainReader.GetHead()
	view, err := walletAPI.walletModule.Chain.ChainReader.StateView(head)
	if err != nil {
		return nil, err
	}
	keyAddr, err := view.ResolveToKeyAddr(ctx, k)
	if err != nil {
		return nil, xerrors.Errorf("failed to resolve ID address: %v", keyAddr)
	}
	return walletAPI.adapter.WalletSign(ctx, keyAddr, msg, meta)
}

func (walletAPI *WalletAPI) WalletSignMessage(ctx context.Context, k address.Address, msg *types.UnsignedMessage) (*types.SignedMessage, error) {
	mb, err := msg.ToStorageBlock()
	if err != nil {
		return nil, xerrors.Errorf("serializing message: %w", err)
	}

	sign, err := walletAPI.WalletSign(ctx, k, mb.Cid().Bytes(), wallet.MsgMeta{Type: core.MTChainMsg})
	if err != nil {
		return nil, xerrors.Errorf("failed to sign message: %w", err)
	}

	return &types.SignedMessage{
		Message:   *msg,
		Signature: *sign,
	}, nil
}

func (walletAPI *WalletAPI) Locked(ctx context.Context, password string) error {
	return walletAPI.walletModule.Wallet.Locked(password)
}

func (walletAPI *WalletAPI) UnLocked(ctx context.Context, password string) error {
	return walletAPI.walletModule.Wallet.UnLocked(password)
}

func (walletAPI *WalletAPI) SetPassword(Context context.Context, password string) error {
	return walletAPI.walletModule.Wallet.SetPassword(password)
}

func (walletAPI *WalletAPI) HasPassword(Context context.Context) bool {
	return walletAPI.adapter.HasPassword()
}

func (walletAPI *WalletAPI) WalletState(Context context.Context) int {
	return walletAPI.walletModule.Wallet.WalletState()
}
