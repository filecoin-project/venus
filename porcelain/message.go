package porcelain

import (
	"context"
	"time"

	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	logging "gx/ipfs/QmcuXC5cxs79ro2cUuHs4HQ2bkDLJUYokwL8aivcX6HW3C/go-log"

	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
	vmErrors "github.com/filecoin-project/go-filecoin/vm/errors"
)

// ErrNoDefaultFromAddress is returned when a default address to send from couldn't be determined (eg, there are zero addresses in the wallet).
var ErrNoDefaultFromAddress = errors.New("unable to determine a default address to send the message from")

// mswrAPI is the subset of the plumbing.API that MessageSendWithRetry uses.
type mswrAPI interface {
	MessageSendWithDefaultAddress(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error)
	MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error
}

var log = logging.Logger("porcelain") // nolint: deadcode

// MessageSendWithRetry sends a message and retries if it does not appear on chain. It returns
// an error if it maxes out its retries, if the message on chain has an error receipt, or if
// it hits an unexpected error.
//
// Note: this is not an awesome retry pattern because:
// - the message could be successfully sent twice eg if attempt 1 appears to fail
//   we will stop waiting for it and attempt a second send. If the first attempt
//   succeeds we don't see it and continue to retry the second.
// - it spams the network with re-tries and doesn't clean up the message pool with
//   attempts that have been abandoned.
// - it's hard to know how long to wait given the potential for null blocks
// - it spends a lot of time walking back in the chain for a message that will not be there.
//   We don't have a mechanism to just wait on future messages or to limit the walk-back.
//
// Access to failures might make this a little better but really the solution is
// to not have re-tries, eg if we had nonce lanes.
func MessageSendWithRetry(ctx context.Context, plumbing mswrAPI, numRetries uint, waitDuration time.Duration, from, to address.Address, val *types.AttoFIL, method string, gasPrice types.AttoFIL, gasLimit types.GasUnits, params ...interface{}) (err error) {
	for i := 0; i < int(numRetries); i++ {
		log.Debugf("SendMessageAndWait (%s) retry %d/%d, waitDuration %v", method, i, numRetries, waitDuration)

		if err = ctx.Err(); err != nil {
			return
		}

		msgCid, err := plumbing.MessageSendWithDefaultAddress(
			ctx,
			from,
			to,
			val,
			gasPrice,
			gasLimit,
			method,
			params...,
		)
		if err != nil {
			return errors.Wrap(err, "couldn't send message")
		}

		waitCtx, waitCancel := context.WithDeadline(ctx, time.Now().Add(waitDuration))
		found := false
		err = plumbing.MessageWait(waitCtx, msgCid, func(blk *types.Block, smsg *types.SignedMessage, receipt *types.MessageReceipt) error {
			found = true
			if receipt.ExitCode != uint8(0) {
				return vmErrors.VMExitCodeToError(receipt.ExitCode, miner.Errors)
			}
			return nil
		})
		waitCancel()
		if err != nil && err != context.DeadlineExceeded {
			return errors.Wrap(err, "got an error from MessageWait")
		} else if found {
			return nil
		}
		// TODO should probably remove the old message from the message queue?
		// TODO could keep a list of preivous message cids in case one of them shows up.
	}

	return errors.Wrapf(err, "failed to send message after waiting %v for each of %d retries ", waitDuration, numRetries)
}

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
