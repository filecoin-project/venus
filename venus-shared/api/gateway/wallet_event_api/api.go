package wallet_event_api

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/types/gateway"
)

type IWalletEventAPI interface {
	ResponseWalletEvent(ctx context.Context, resp *gateway.ResponseEvent) error                                        //perm:write
	ListenWalletEvent(ctx context.Context, policy *gateway.WalletRegisterPolicy) (<-chan *gateway.RequestEvent, error) //perm:write
	SupportNewAccount(ctx context.Context, channelId types.UUID, account string) error                                 //perm:write
	AddNewAddress(ctx context.Context, channelId types.UUID, newAddrs []address.Address) error                         //perm:write
	RemoveAddress(ctx context.Context, channelId types.UUID, newAddrs []address.Address) error                         //perm:write
}
