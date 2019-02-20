package porcelain

import (
	"context"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	logging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

// ErrNoDefaultFromAddress is returned when a default address to send from couldn't be determined (eg, there are zero addresses in the wallet).
var ErrNoDefaultFromAddress = errors.New("unable to determine a default address to send the message from")

var log = logging.Logger("porcelain") // nolint: deadcode

// mswdaAPI is the subset of the plumbing.API that MessageSendWithDefaultAddress uses.
type mswdaAPI interface {
	GetAndMaybeSetDefaultSenderAddress() (address.Address, error)
	MessageSend(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error)
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
	if from == (address.Address{}) {
		ret, err := plumbing.GetAndMaybeSetDefaultSenderAddress()
		if (err != nil && err == ErrNoDefaultFromAddress) || ret == (address.Address{}) {
			return cid.Undef, ErrNoDefaultFromAddress
		}
		from = ret
	}

	return plumbing.MessageSend(ctx, from, to, value, gasPrice, gasLimit, method, params...)
}

// gamsdsaAPI is the subset of the plumbing.API that GetAndMaybeSetDefaultSenderAddress uses.
type gamsdsaAPI interface {
	ConfigGet(dottedPath string) (interface{}, error)
	ConfigSet(dottedPath string, paramJSON string) error

	WalletAddresses() []address.Address
}

// GetAndMaybeSetDefaultSenderAddress returns a default address from which to
// send messsages. If none is set it picks the first address in the wallet and
// sets it as the default in the config.
func GetAndMaybeSetDefaultSenderAddress(plumbing gamsdsaAPI) (address.Address, error) {
	ret, err := plumbing.ConfigGet("wallet.defaultAddress")
	addr := ret.(address.Address)
	if err != nil || addr != (address.Address{}) {
		return addr, err
	}

	// No default is set; pick the 0th and make it the default.
	if len(plumbing.WalletAddresses()) > 0 {
		addr := plumbing.WalletAddresses()[0]
		err := plumbing.ConfigSet("wallet.defaultAddress", addr.String())
		if err != nil {
			return address.Address{}, err
		}

		return addr, nil
	}

	return address.Address{}, ErrNoDefaultFromAddress
}
