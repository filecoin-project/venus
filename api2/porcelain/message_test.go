package porcelain_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	hamt "gx/ipfs/QmRXf2uUSdGSunRJsM9wXSUNVwLUGCY3So5fAs7h2CBJVf/go-hamt-ipld"
	blockstore "gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"
	datastore "gx/ipfs/Qmf4xQhNomPNhrtZc67qSnfJSjxjXs9LWvknJtSXwimPrM/go-datastore"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/api2/impl/mthdsigapi"
	"github.com/filecoin-project/go-filecoin/api2/porcelain"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// type plumbing interface {
// 	ActorGetSignature(ctx context.Context, actorAddr address.Address, method string) (*exec.FunctionSignature, error)
// 	MessageSend(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasCost, method string, params ...interface{}) (cid.Cid, error)
// 	MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error
// }

var newCid = types.NewCidForTestGetter()
var newAddr = address.NewForTestGetter()

type fakeChainReadStore struct {
	st state.Tree
}

func (f *fakeChainReadStore) LatestState(ctx context.Context) (state.Tree, error) {
	return f.st, nil
}

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
	return nil
}

func TestMessageSendWithRetry(t *testing.T) {
	t.Parallel()
	val, gasPrice, gasLimit := types.NewAttoFILFromFIL(0), types.NewGasPrice(0), types.NewGasCost(0)

	// t.Run("succeeds on first try", func(t *testing.T) {
	// 	require := require.New(t)
	// 	assert := assert.New(t)
	// 	ctx := context.Background()
	// 	from, to := newAddr(), newAddr()

	// 	fp := &fakePlumbing{assert: assert, require: require}
	// 	fp.actorGetSignature = fp.nopActorGetSignature
	// 	fp.messageSend = fp.successfulMessageSend
	// 	fp.messageWait = fp.successfulMessageWait

	// 	_, err := porcelain.MessageSendWithRetry(ctx, fp, 10 /* retries */, 1*time.Second /* wait time*/, from, to, val, "", gasPrice, gasLimit)
	// 	require.NoError(err)
	// 	assert.Equal(1, fp.sendCnt)
	// })

	// t.Run("retries if not successful", func(t *testing.T) {
	// 	require := require.New(t)
	// 	assert := assert.New(t)
	// 	ctx := context.Background()
	// 	from, to := newAddr(), newAddr()

	// 	fp := &fakePlumbing{assert: assert, require: require}
	// 	fp.actorGetSignature = fp.nopActorGetSignature
	// 	fp.messageSend = fp.successfulMessageSend
	// 	fp.messageWait = fp.unsuccessfulMessageWait

	// 	_, err := porcelain.MessageSendWithRetry(ctx, fp, 10 /* retries */, 1*time.Second /* wait time*/, from, to, val, "", gasPrice, gasLimit)
	// 	require.NoError(err)
	// 	assert.Equal(10, fp.sendCnt)
	// })

	// t.Run("respects top-level context", func(t *testing.T) {
	// 	require := require.New(t)
	// 	assert := assert.New(t)
	// 	ctx, cancel := context.WithCancel(context.Background())
	// 	from, to := newAddr(), newAddr()

	// 	fp := &fakePlumbing{assert: assert, require: require}
	// 	fp.actorGetSignature = fp.nopActorGetSignature
	// 	fp.messageSend = fp.successfulMessageSend
	// 	// This MessageWait cancels and ctx and returns unsuccessfully. The effect is
	// 	// canceling the global context during the first run; we expect it not to retry
	// 	// if that is the case ie sendCnt to be 1.
	// 	fp.messageWait = func(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	// 		cancel()
	// 		return nil
	// 	}

	// 	_, err := porcelain.MessageSendWithRetry(ctx, fp, 10 /* retries */, 1*time.Second /* wait time*/, from, to, val, "", gasPrice, gasLimit)
	// 	require.Error(err)
	// 	assert.Equal(1, fp.sendCnt)
	// })

	t.Run("returns sane message receipt results", func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)
		ctx := context.Background()
		from := newAddr()

		// We're gonna use a real signature getter so set up a To actor
		// that has a method with a non-trivial signature.
		// First, get a signed message and pull out the To field.
		ki := types.MustGenerateKeyInfo(10, types.GenerateKeyInfoSeed())
		mockSigner := types.NewMockSigner(ki)
		signedMessage := types.NewSignedMessageForTestGetter(mockSigner)()
		signedMessage.Method = "hasReturnValue"
		to := signedMessage.To
		// Now, install an actor at that address.
		cst := hamt.NewCborStore()
		bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
		vms := vm.NewStorageMap(bs)
		fakeActorCodeCid := newCid()
		builtin.Actors[fakeActorCodeCid] = &actor.FakeActor{}
		defer func() {
			delete(builtin.Actors, fakeActorCodeCid)
		}()
		fakeActor := th.RequireNewFakeActorWithTokens(require, vms, to, fakeActorCodeCid, types.NewAttoFILFromFIL(102))
		_, st := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
			to: fakeActor,
		})
		sigGetter := mthdsigapi.NewGetter(&fakeChainReadStore{st})

		fp := &fakePlumbing{assert: assert, require: require}
		fp.actorGetSignature = sigGetter.Get
		fp.messageSend = fp.successfulMessageSend
		expectedAddr := newAddr()
		expectedAddrEncoded, err := abi.ToEncodedValues(expectedAddr)
		require.NoError(err)
		expectedAddrBytes := []types.Bytes{expectedAddrEncoded}
		fmt.Printf("\nex=%v\n", expectedAddrBytes)
		fp.messageWait = func(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
			return cb(&types.Block{}, signedMessage, &types.MessageReceipt{ExitCode: 0, Return: expectedAddrBytes})
		}

		res, err := porcelain.MessageSendWithRetry(ctx, fp, 10 /* retries */, 1*time.Second /* wait time*/, from, to, val, "hasReturnValue", gasPrice, gasLimit)
		require.NoError(err)
		require.Equal(1, len(res))
		fmt.Printf("\nres=%T\n", res)
		fmt.Printf("\nres[0]=%T\n", res[0])
		res0 := res[0]
		require.Equal(1, len(res0))
		gotAddr, ok := res0[0].(address.Address)
		assert.True(ok)
		assert.Equal(expectedAddr, gotAddr)
	})

	// t.Run("succeeds if method exists", func(t *testing.T) {
	// 	require := require.New(t)

	// 	ctx := context.Background()
	// 	cst := hamt.NewCborStore()
	// 	addr := address.NewForTestGetter()()
	// 	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	// 	vms := vm.NewStorageMap(bs)

	// 	// Install the fake actor so we can query one of its method signatures.
	// 	emptyActorCodeCid := types.NewCidForTestGetter()()
	// 	builtin.Actors[emptyActorCodeCid] = &actor.FakeActor{}
	// 	defer func() {
	// 		delete(builtin.Actors, emptyActorCodeCid)
	// 	}()

	// 	fakeActor := th.RequireNewFakeActorWithTokens(require, vms, addr, emptyActorCodeCid, types.NewAttoFILFromFIL(102))
	// 	_, st := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
	// 		addr: fakeActor,
	// 	})
	// 	getter := mthdsigapi.NewGetter(&fakeChainReadStore{st})

	// 	sig, err := getter.Get(ctx, addr, "hasReturnValue")
	// 	require.NoError(err)
	// 	expected := &exec.FunctionSignature{Params: []abi.Type(nil), Return: []abi.Type{abi.Address}}
	// 	require.Equal(expected, sig)
	// })

	// t.Run("errors if no such method", func(t *testing.T) {
	// 	require := require.New(t)

	// 	ctx := context.Background()
	// 	cst := hamt.NewCborStore()
	// 	addr := address.NewForTestGetter()()

	// 	acctActor := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(10000))
	// 	_, st := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
	// 		addr: acctActor,
	// 	})

	// 	getter := mthdsigapi.NewGetter(&fakeChainReadStore{st})

	// 	_, err := getter.Get(ctx, addr, "NoSuchMethod")
	// 	require.Error(err)
	// })

	// t.Run("errors with ErrNoMethod if no method", func(t *testing.T) {
	// 	assert := assert.New(t)
	// 	require := require.New(t)

	// 	ctx := context.Background()
	// 	cst := hamt.NewCborStore()
	// 	addr := address.NewForTestGetter()()

	// 	acctActor := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(10000))
	// 	_, st := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
	// 		addr: acctActor,
	// 	})

	// 	getter := mthdsigapi.NewGetter(&fakeChainReadStore{st})

	// 	sig, err := getter.Get(ctx, addr, "")
	// 	assert.Equal(mthdsigapi.ErrNoMethod, err)
	// 	assert.Nil(sig)
	// })

	// t.Run("errors if actor undefined", func(t *testing.T) {
	// 	require := require.New(t)

	// 	ctx := context.Background()
	// 	cst := hamt.NewCborStore()
	// 	addr := address.NewForTestGetter()()

	// 	// Install the empty actor so we can query one of its method signatures.
	// 	emptyActorCodeCid := types.NewCidForTestGetter()()
	// 	builtin.Actors[emptyActorCodeCid] = &actor.FakeActor{}
	// 	defer func() {
	// 		delete(builtin.Actors, emptyActorCodeCid)
	// 	}()
	// 	emptyActor := th.RequireNewEmptyActor(require, types.NewAttoFILFromFIL(0))

	// 	_, st := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
	// 		addr: emptyActor,
	// 	})

	// 	getter := mthdsigapi.NewGetter(&fakeChainReadStore{st})

	// 	_, err := getter.Get(ctx, addr, "Foo")
	// 	require.Error(err)
	// })
}
