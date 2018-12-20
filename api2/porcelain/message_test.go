package porcelain_test

import (
	"context"
	"testing"
	"time"

	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/api2/porcelain"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var newCid = types.NewCidForTestGetter()
var newAddr = address.NewForTestGetter()

type fakePlumbing struct {
	assert  *assert.Assertions
	require *require.Assertions

	msgCid  cid.Cid
	sendCnt int

	actorGetSignature func(ctx context.Context, actorAddr address.Address, method string) (*exec.FunctionSignature, error)
	messageSend       func(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasCost, method string, params ...interface{}) (cid.Cid, error)
	messageWait       func(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error
}

func (fp *fakePlumbing) ActorGetSignature(ctx context.Context, actorAddr address.Address, method string) (*exec.FunctionSignature, error) {
	return fp.actorGetSignature(ctx, actorAddr, method)
}

func (fp *fakePlumbing) MessageSend(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasCost, method string, params ...interface{}) (cid.Cid, error) {
	return fp.messageSend(ctx, from, to, value, gasPrice, gasLimit, method, params...)
}

func (fp *fakePlumbing) MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	return fp.messageWait(ctx, msgCid, cb)
}

func (fp *fakePlumbing) nopActorGetSignature(ctx context.Context, actorAddr address.Address, method string) (*exec.FunctionSignature, error) {
	return &exec.FunctionSignature{}, nil
}

func (fp *fakePlumbing) successfulMessageSend(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasCost, method string, params ...interface{}) (cid.Cid, error) {
	fp.msgCid = newCid()
	fp.sendCnt++
	return fp.msgCid, nil
}

func (fp *fakePlumbing) successfulMessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	fp.require.NotEqual(cid.Undef, fp.msgCid)
	fp.assert.True(fp.msgCid.Equals(msgCid))
	cb(&types.Block{}, &types.SignedMessage{}, &types.MessageReceipt{ExitCode: 0, Return: []types.Bytes{}})
	return nil
}

func (fp *fakePlumbing) unsuccessfulMessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	fp.require.NotEqual(cid.Undef, fp.msgCid)
	fp.assert.True(fp.msgCid.Equals(msgCid))
	return context.DeadlineExceeded
}

func TestMessageSendWithRetry(t *testing.T) {
	t.Parallel()
	val, gasPrice, gasLimit := types.NewAttoFILFromFIL(0), types.NewGasPrice(0), types.NewGasCost(0)

	t.Run("succeeds on first try", func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)
		ctx := context.Background()
		from, to := newAddr(), newAddr()

		fp := &fakePlumbing{assert: assert, require: require}
		fp.actorGetSignature = fp.nopActorGetSignature
		fp.messageSend = fp.successfulMessageSend
		fp.messageWait = fp.successfulMessageWait

		err := porcelain.MessageSendWithRetry(ctx, fp, 10 /* retries */, 1*time.Second /* wait time*/, from, to, val, "", gasPrice, gasLimit)
		require.NoError(err)
		assert.Equal(1, fp.sendCnt)
	})

	t.Run("retries if not successful", func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)
		ctx := context.Background()
		from, to := newAddr(), newAddr()

		fp := &fakePlumbing{assert: assert, require: require}
		fp.actorGetSignature = fp.nopActorGetSignature
		fp.messageSend = fp.successfulMessageSend
		fp.messageWait = fp.unsuccessfulMessageWait

		err := porcelain.MessageSendWithRetry(ctx, fp, 10 /* retries */, 1*time.Second /* wait time*/, from, to, val, "", gasPrice, gasLimit)
		require.NoError(err)
		assert.Equal(10, fp.sendCnt)
	})

	t.Run("respects top-level context", func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)
		ctx, cancel := context.WithCancel(context.Background())
		from, to := newAddr(), newAddr()

		fp := &fakePlumbing{assert: assert, require: require}
		fp.actorGetSignature = fp.nopActorGetSignature
		fp.messageSend = fp.successfulMessageSend
		// This MessageWait cancels and ctx and returns unsuccessfully. The effect is
		// canceling the global context during the first run; we expect it not to retry
		// if that is the case ie sendCnt to be 1.
		fp.messageWait = func(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
			cancel()
			return nil
		}

		err := porcelain.MessageSendWithRetry(ctx, fp, 10 /* retries */, 1*time.Second /* wait time*/, from, to, val, "", gasPrice, gasLimit)
		require.Error(err)
		assert.Equal(1, fp.sendCnt)
	})
}
