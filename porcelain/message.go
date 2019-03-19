package porcelain

import (
	"context"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	logging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

var log = logging.Logger("porcelain") // nolint: deadcode

// mswdaAPI is the subset of the plumbing.API that MessageSendWithDefaultAddress uses.
type mswdaAPI interface {
	MessageSend(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error)
	WalletDefaultAddress() (address.Address, error)
}

// MessageSendWithDefaultAddress calls MessageSend but with a default from
// address if none is provided. If you don't need a default address provided,
// use MessageSend instead.
func MessageSendWithDefaultAddress(
	ctx context.Context,
	plumbing mswdaAPI,
	from,
	to address.Address,
	value *types.AttoFIL,
	gasPrice types.AttoFIL,
	gasLimit types.GasUnits,
	method string,
	params ...interface{},
) (cid.Cid, error) {
	// If the from address isn't set attempt to use the default address.
	if from.Empty() {
		ret, err := plumbing.WalletDefaultAddress()
		if (err != nil && err == ErrNoDefaultFromAddress) || ret.Empty() {
			return cid.Undef, ErrNoDefaultFromAddress
		}
		from = ret
	}

	return plumbing.MessageSend(ctx, from, to, value, gasPrice, gasLimit, method, params...)
}
