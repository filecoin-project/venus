package wallet

import (
	"context"
	"errors"
	"strings"

	"github.com/filecoin-project/venus/app/submodule/wallet/remotewallet"
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
	WalletBalance(ctx context.Context, addr address.Address) (abi.TokenAmount, error) //not exists in remote
	WalletHas(ctx context.Context, addr address.Address) (bool, error)
	WalletDefaultAddress() (address.Address, error) //not exists in remote
	WalletAddresses() []address.Address
	WalletSetDefault(_ context.Context, addr address.Address) error //not exists in remote
	WalletNewAddress(protocol address.Protocol) (address.Address, error)
	WalletImport(key *crypto.KeyInfo) (address.Address, error)
	WalletExport(addr address.Address, password string) (*crypto.KeyInfo, error)
	WalletSign(ctx context.Context, k address.Address, msg []byte, meta wallet.MsgMeta) (*crypto.Signature, error)
	WalletSignMessage(ctx context.Context, k address.Address, msg *types.UnsignedMessage) (*types.SignedMessage, error)
}

var ErrNoDefaultFromAddress = errors.New("unable to determine a default walletModule address")

type WalletAPI struct { //nolint
	walletModule *WalletSubmodule
	remote       *remotewallet.RemoteWallet
	isRemote     bool
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
	if walletAPI.isRemote {
		return walletAPI.remote.WalletHas(ctx, addr)
	}
	return walletAPI.walletModule.Wallet.HasAddress(addr), nil
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
	if walletAPI.isRemote {
		wallets, err := walletAPI.remote.WalletList(context.Background())
		if err != nil {
			return make([]address.Address, 0)
		}
		return wallets
	}
	return walletAPI.walletModule.Wallet.Addresses()
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
	if walletAPI.isRemote {
		return walletAPI.remote.WalletNew(context.Background(), remotewallet.GetKeyType(protocol))
	}
	return walletAPI.walletModule.Wallet.NewAddress(protocol)
}

// WalletImport adds a given set of KeyInfos to the walletModule
func (walletAPI *WalletAPI) WalletImport(key *crypto.KeyInfo) (address.Address, error) {
	if walletAPI.isRemote {
		return walletAPI.remote.WalletImport(context.Background(), remotewallet.ConvertRemoteKeyInfo(key))
	}

	addr, err := walletAPI.walletModule.Wallet.Import(key)
	if err != nil {
		return address.Undef, err
	}
	return addr, nil
}

// WalletExport returns the KeyInfos for the given walletModule addresses
func (walletAPI *WalletAPI) WalletExport(addr address.Address, password string) (*crypto.KeyInfo, error) {
	if walletAPI.isRemote {
		// todo: Implement password logic in the future
		key, err := walletAPI.remote.WalletExport(context.Background(), addr)
		if err != nil {
			return nil, err
		}
		return remotewallet.ConvertLocalKeyInfo(key), nil
	}
	return walletAPI.walletModule.Wallet.Export(addr, password)
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

	if walletAPI.isRemote {
		return walletAPI.remote.WalletSign(ctx, keyAddr, msg, meta)
	}
	return walletAPI.walletModule.Wallet.WalletSign(ctx, keyAddr, msg, meta)
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

func (walletAPI *WalletAPI) HavePassword(Context context.Context) bool {
	return walletAPI.walletModule.Wallet.HavePassword()
}

func (walletAPI *WalletAPI) WalletState(Context context.Context) int {
	return walletAPI.walletModule.Wallet.WalletState()
}
