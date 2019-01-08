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

// mswrPlumbing is the subset of the plumbing.API that MessageSendWithRetry uses.
type mswrPlumbing interface {
	MessageSend(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasCost, method string, params ...interface{}) (cid.Cid, error)
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
func MessageSendWithRetry(ctx context.Context, plumbing mswrPlumbing, numRetries uint, waitDuration time.Duration, from, to address.Address, val *types.AttoFIL, method string, gasPrice types.AttoFIL, gasLimit types.GasCost, params ...interface{}) (err error) {
	for i := 0; i < int(numRetries); i++ {
		log.Debugf("SendMessageAndWait (%s) retry %d/%d, waitDuration %v", method, i, numRetries, waitDuration)

		if err = ctx.Err(); err != nil {
			return
		}

		msgCid, err := plumbing.MessageSend(ctx, from, to, val, gasPrice, gasLimit, method, params...)
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
