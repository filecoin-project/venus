package gateway

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/venus/pkg/crypto"
	gatewayAPI "github.com/filecoin-project/venus/venus-shared/api/gateway/v2"
	"github.com/filecoin-project/venus/venus-shared/types"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("wallet-gateway")

type WalletGateway struct {
	cli    gatewayAPI.IWalletClient
	closer jsonrpc.ClientCloser

	addressAccount map[address.Address][]string

	lk sync.RWMutex
}

func NewWalletGateway(ctx context.Context, url, token string) (*WalletGateway, error) {
	cli, close, err := gatewayAPI.DialIGatewayRPC(ctx, url, token, nil)
	if err != nil {
		return nil, err
	}

	wg := &WalletGateway{cli: cli, closer: close, addressAccount: map[address.Address][]string{}}
	err = wg.updateAddressAccount(ctx)
	if err != nil {
		return nil, err
	}
	go wg.loopUpdateAddressAccount(ctx)

	return wg, nil
}

func (w *WalletGateway) updateAddressAccount(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	w.lk.Lock()
	defer w.lk.Unlock()

	wds, err := w.cli.ListWalletInfo(ctx)
	if err != nil {
		return err
	}

	for _, wd := range wds {
		accountMap := map[string]struct{}{
			wd.Account: {},
		}
		accounts := []string{wd.Account}
		for _, account := range wd.SupportAccounts {
			if _, ok := accountMap[account]; ok {
				continue
			}
			accounts = append(accounts, account)
			accountMap[account] = struct{}{}
		}

		addrs := make(map[address.Address]struct{}, 0)
		for _, cs := range wd.ConnectStates {
			for _, addr := range cs.Addrs {
				addrs[addr] = struct{}{}
			}
		}

		for addr := range addrs {
			w.addressAccount[addr] = accounts
		}
	}

	return err
}

func (w *WalletGateway) loopUpdateAddressAccount(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := w.updateAddressAccount(ctx)
			if err != nil {
				log.Errorf("update address account failed: %s", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (w *WalletGateway) Close() {
	if w == nil {
		return
	}
	w.closer()
}

func (w *WalletGateway) WalletSign(ctx context.Context, k address.Address, msg []byte, meta types.MsgMeta) (*crypto.Signature, error) {
	w.lk.RLock()
	accounts, ok := w.addressAccount[k]
	w.lk.RUnlock()
	if !ok {
		return nil, fmt.Errorf("address %s not found", k)
	}

	return w.cli.WalletSign(ctx, k, accounts, msg, meta)
}
