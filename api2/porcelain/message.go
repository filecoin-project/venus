package porcelain

import (
	"context"
	"fmt"
	"time"

	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	logging "gx/ipfs/QmcuXC5cxs79ro2cUuHs4HQ2bkDLJUYokwL8aivcX6HW3C/go-log"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/api2/impl/mthdsigapi"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
	vmErrors "github.com/filecoin-project/go-filecoin/vm/errors"
	"github.com/pkg/errors"
)

type plumbing interface {
	ActorGetSignature(ctx context.Context, actorAddr address.Address, method string) (*exec.FunctionSignature, error)
	MessageSend(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasCost, method string, params ...interface{}) (cid.Cid, error)
	MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error
}

var log = logging.Logger("porcelain") // nolint: deadcode

// SendMessageAndWait creates a message, adds it to the mempool and waits for inclusion.
// It will retry upto retries times, if there is a nonce error when including it.
// It returns the deserialized results, or an error.

// - re-org could bounce out
// Bad idea:
// - hard to know how long to wait bc null blocks
// - could happen twice: retry creates a new one and the old one gets mined
// - can't get access to perm and temp failures, only see if it is onchain; kinda want visibility into failures so react quicker
// really want lanes
// - errors could be fatal
// - spams the network

// if send errors then it errors

// test send error
// test cancel
// test wait timeout
// test signature errors

// bad: if timeout is too low to walk back in chain

// TODO verify Wiat impl deps dont return an error if the context is canceled

func MessageSendWithRetry(ctx context.Context, plumbing plumbing, numRetries uint, waitDuration time.Duration, from, to address.Address, val *types.AttoFIL, method string, gasPrice types.AttoFIL, gasLimit types.GasCost, params ...interface{}) (res []interface{}, err error) {
	for i := 0; i < int(numRetries); i++ {
		log.Debugf("SendMessageAndWait (%s) retry %d/%d, waitDuration %v", method, i, numRetries, waitDuration)

		if err = ctx.Err(); err != nil {
			return
		}

		msgCid, err := plumbing.MessageSend(ctx, from, to, val, gasPrice, gasLimit, method, params...)
		if err != nil {
			return nil, errors.Wrap(err, "couldn't send message")
		}

		waitCtx, waitCancel := context.WithDeadline(ctx, time.Now().Add(waitDuration))
		found := false
		err = plumbing.MessageWait(waitCtx, msgCid, func(blk *types.Block, smsg *types.SignedMessage, receipt *types.MessageReceipt) error {
			found = true

			if receipt.ExitCode != uint8(0) {
				return vmErrors.VMExitCodeToError(receipt.ExitCode, miner.Errors)
			}

			// Note: a better pattern would be to have MessageSendWithRetry just send and retry, and
			// leave the method signature stuff to the caller if in fact they want to do it at all.
			mthdSig, err2 := plumbing.ActorGetSignature(waitCtx, smsg.Message.To, smsg.Message.Method)
			if err2 == mthdsigapi.ErrNoMethod && err2 == mthdsigapi.ErrNoActorImpl {
				fmt.Printf("\n\n-1=%#+v\n\n", receipt)

				return nil
			} else if err2 != nil {
				fmt.Printf("\n\n-2=%#+v\n\n", receipt)
				return err2
			}
			res = make([]interface{}, len(receipt.Return))
			fmt.Printf("\n\n0=%#+v\n\n", receipt)
			for i := 0; i < len(receipt.Return); i++ {
				fmt.Printf("\n\n1\n\n")
				val, err := abi.DecodeValues(receipt.Return[i], []abi.Type{mthdSig.Return[i]})
				if err != nil {
					return err
				}
				res[i] = abi.FromValues(val)
				fmt.Printf("\nres[0]=%v\n", res[i])
			}
			return nil
		})
		waitCancel()
		fmt.Printf("\nerr = %v\n", err)
		if err != nil {
			return nil, errors.Wrap(err, "got an error from MessageWait")
		} else if found {
			return res, nil
		}

		// cleanup message pool ?????
		//node.MsgPool.Remove(msgCid)
	}

	return nil, errors.Wrapf(err, "failed to send message after waiting %v for each of %d retries ", waitDuration, numRetries)
}
