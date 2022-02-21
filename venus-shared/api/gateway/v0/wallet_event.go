package gateway

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/crypto"

	"github.com/filecoin-project/venus/venus-shared/types"
	gtypes "github.com/filecoin-project/venus/venus-shared/types/gateway"
)

type IWalletEvent interface {
	IWalletClient
	IWalletServiceProvider
}

type IWalletClient interface {
	ListWalletInfo(ctx context.Context) ([]*gtypes.WalletDetail, error)                                                                 //perm:admin
	ListWalletInfoByWallet(ctx context.Context, wallet string) (*gtypes.WalletDetail, error)                                            //perm:admin
	WalletHas(ctx context.Context, supportAccount string, addr address.Address) (bool, error)                                           //perm:admin
	WalletSign(ctx context.Context, account string, addr address.Address, toSign []byte, meta types.MsgMeta) (*crypto.Signature, error) //perm:admin

}

type IWalletServiceProvider interface {
	ResponseWalletEvent(ctx context.Context, resp *gtypes.ResponseEvent) error                                       //perm:read
	ListenWalletEvent(ctx context.Context, policy *gtypes.WalletRegisterPolicy) (<-chan *gtypes.RequestEvent, error) //perm:read
	SupportNewAccount(ctx context.Context, channelID types.UUID, account string) error                               //perm:read
	AddNewAddress(ctx context.Context, channelID types.UUID, newAddrs []address.Address) error                       //perm:read
	RemoveAddress(ctx context.Context, channelID types.UUID, newAddrs []address.Address) error                       //perm:read
}
