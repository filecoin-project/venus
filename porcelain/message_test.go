package porcelain

import (
	"context"
	"testing"
	"time"

	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/plumbing/cfg"
	"github.com/filecoin-project/go-filecoin/plumbing/wallet"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var newCid = types.NewCidForTestGetter()
var newAddr = address.NewForTestGetter()

type fakePlumbing struct {
	config *cfg.Config
	wallet *wallet.Wallet

	assert  *assert.Assertions
	require *require.Assertions

	msgCid  cid.Cid
	sendCnt int

	messageSend func(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error)
	messageWait func(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error
}

// Satisfy the plumbing API:

func (fp *fakePlumbing) MessageQuery(ctx context.Context, from, to address.Address, method string, params ...interface{}) ([][]byte, *exec.FunctionSignature, error) {
	return nil, nil, nil
}

func (fp *fakePlumbing) MessagePreview(ctx context.Context, from, to address.Address, method string, params ...interface{}) (types.GasUnits, error) {
	return types.NewGasUnits(0), nil
}

func (fp *fakePlumbing) MessageSend(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
	return fp.messageSend(ctx, from, to, value, gasPrice, gasLimit, method, params...)
}

func (fp *fakePlumbing) MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	return fp.messageWait(ctx, msgCid, cb)
}

func (fp *fakePlumbing) ConfigSet(dottedKey string, jsonString string) error {
	return fp.config.Set(dottedKey, jsonString)
}

func (fp *fakePlumbing) ConfigGet(dottedPath string) (interface{}, error) {
	return fp.config.Get(dottedPath)
}

func (fp *fakePlumbing) WalletAddresses() []address.Address {
	return fp.wallet.Addresses()
}

func (fp *fakePlumbing) WalletNewAddress() (address.Address, error) {
	return fp.wallet.NewAddress()
}

// Fake implementations we'll use.
func (fp *fakePlumbing) successfulMessageSend(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
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
	val, gasPrice, gasLimit := types.NewAttoFILFromFIL(0), types.NewGasPrice(0), types.NewGasUnits(0)

	t.Run("succeeds on first try", func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)
		ctx := context.Background()
		from, to := newAddr(), newAddr()

		fp := &fakePlumbing{assert: assert, require: require}
		fp.messageSend = fp.successfulMessageSend
		fp.messageWait = fp.successfulMessageWait

		err := MessageSendWithRetry(ctx, fp, 10 /* retries */, 1*time.Second /* wait time*/, from, to, val, "", gasPrice, gasLimit)
		require.NoError(err)
		assert.Equal(1, fp.sendCnt)
	})

	t.Run("retries if not successful", func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)
		ctx := context.Background()
		from, to := newAddr(), newAddr()

		fp := &fakePlumbing{assert: assert, require: require}
		fp.messageSend = fp.successfulMessageSend
		fp.messageWait = fp.unsuccessfulMessageWait

		err := MessageSendWithRetry(ctx, fp, 10 /* retries */, 1*time.Second /* wait time*/, from, to, val, "", gasPrice, gasLimit)
		require.NoError(err)
		assert.Equal(10, fp.sendCnt)
	})

	t.Run("respects top-level context", func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)
		ctx, cancel := context.WithCancel(context.Background())
		from, to := newAddr(), newAddr()

		fp := &fakePlumbing{assert: assert, require: require}
		fp.messageSend = fp.successfulMessageSend
		// This MessageWait cancels and ctx and returns unsuccessfully. The effect is
		// canceling the global context during the first run; we expect it not to retry
		// if that is the case ie sendCnt to be 1.
		fp.messageWait = func(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
			cancel()
			return nil
		}

		err := MessageSendWithRetry(ctx, fp, 10 /* retries */, 1*time.Second /* wait time*/, from, to, val, "", gasPrice, gasLimit)
		require.Error(err)
		assert.Equal(1, fp.sendCnt)
	})
}

func TestGetAndMaybeSetDefaultSenderAddress(t *testing.T) {
	t.Parallel()

	t.Run("it returns the configured wallet default if it exists", func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)

		fp := newMinerTestPlumbing(assert, require)

		addrA, err := fp.WalletNewAddress()
		require.NoError(err)
		fp.ConfigSet("wallet.defaultAddress", addrA.String())

		addrB, err := GetAndMaybeSetDefaultSenderAddress(fp)
		require.NoError(err)
		require.Equal(addrA.String(), addrB.String())
	})

	t.Run("default is consistent if none configured", func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)

		fp := newMinerTestPlumbing(assert, require)

		addresses := []address.Address{}
		for i := 0; i < 10; i++ {
			a, err := fp.WalletNewAddress()
			require.NoError(err)
			addresses = append(addresses, a)
		}

		expected, err := GetAndMaybeSetDefaultSenderAddress(fp)
		require.NoError(err)
		require.True(isInList(expected, addresses))
		for i := 0; i < 30; i++ {
			got, err := GetAndMaybeSetDefaultSenderAddress(fp)
			require.NoError(err)
			require.Equal(expected, got)
		}
	})
}

func isInList(needle address.Address, haystack []address.Address) bool {
	for _, a := range haystack {
		if a == needle {
			return true
		}
	}
	return false
}
