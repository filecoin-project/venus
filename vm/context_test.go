package vm

import (
	"context"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	xerrors "github.com/pkg/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/state"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm/errors"
)

func TestVMContextStorage(t *testing.T) {
	tf.UnitTest(t)

	addrGetter := address.NewForTestGetter()
	ctx := context.Background()

	cst := hamt.NewCborStore()
	st := state.NewEmptyStateTree(cst)
	cstate := state.NewCachedStateTree(st)

	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	vms := NewStorageMap(bs)

	toActor, err := account.NewActor(types.ZeroAttoFIL)
	assert.NoError(t, err)
	toAddr := addrGetter()

	assert.NoError(t, st.SetActor(ctx, toAddr, toActor))
	msg := types.NewMessage(addrGetter(), toAddr, 0, types.ZeroAttoFIL, "hello", nil)

	to, err := cstate.GetActor(ctx, toAddr)
	assert.NoError(t, err)
	vmCtxParams := NewContextParams{
		From:        nil,
		To:          to,
		Message:     msg,
		State:       cstate,
		StorageMap:  vms,
		GasTracker:  NewGasTracker(),
		BlockHeight: types.NewBlockHeight(0),
	}
	vmCtx := NewVMContext(vmCtxParams)

	node, err := cbor.WrapObject([]byte("hello"), types.DefaultHashFunction, -1)
	assert.NoError(t, err)

	cid, err := vmCtx.Storage().Put(node.RawData())
	require.NoError(t, err)
	err = vmCtx.Storage().Commit(cid, vmCtx.Storage().Head())
	require.NoError(t, err)
	assert.NoError(t, cstate.Commit(ctx))

	// make sure we can read it back
	toActorBack, err := st.GetActor(ctx, toAddr)
	assert.NoError(t, err)
	vmCtxParams.To = toActorBack
	storage, err := vmCtx.Storage().Get(vmCtx.Storage().Head())
	assert.NoError(t, err)
	assert.Equal(t, storage, node.RawData())
}

func TestVMContextSendFailures(t *testing.T) {
	tf.UnitTest(t)

	actor1 := actor.NewActor(cid.Undef, types.NewAttoFILFromFIL(100))
	actor2 := actor.NewActor(cid.Undef, types.NewAttoFILFromFIL(50))
	newMsg := types.NewMessageForTestGetter()
	newAddress := address.NewForTestGetter()

	mockStateTree := state.MockStateTree{
		BuiltinActors: map[cid.Cid]exec.ExecutableActor{},
	}
	fakeActorCid := types.NewCidForTestGetter()()
	mockStateTree.BuiltinActors[fakeActorCid] = &actor.FakeActor{}
	tree := state.NewCachedStateTree(&mockStateTree)
	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	vms := NewStorageMap(bs)

	vmCtxParams := NewContextParams{
		From:        actor1,
		To:          actor2,
		Message:     newMsg(),
		State:       tree,
		StorageMap:  vms,
		GasTracker:  NewGasTracker(),
		BlockHeight: types.NewBlockHeight(0),
	}

	t.Run("failure to convert to ABI values results in fault error", func(t *testing.T) {
		var calls []string
		deps := &deps{
			ToValues: func(_ []interface{}) ([]*abi.Value, error) {
				calls = append(calls, "ToValues")
				return nil, xerrors.New("error")
			},
		}

		ctx := NewVMContext(vmCtxParams)
		ctx.deps = deps

		_, code, err := ctx.Send(newAddress(), "foo", types.ZeroAttoFIL, []interface{}{})

		assert.Error(t, err)
		assert.Equal(t, 1, int(code))
		assert.True(t, errors.IsFault(err))
		assert.Equal(t, []string{"ToValues"}, calls)
	})

	t.Run("failure to encode ABI values to byte slice results in revert error", func(t *testing.T) {
		var calls []string
		deps := &deps{
			EncodeValues: func(_ []*abi.Value) ([]byte, error) {
				calls = append(calls, "EncodeValues")
				return nil, xerrors.New("error")
			},
			ToValues: func(_ []interface{}) ([]*abi.Value, error) {
				calls = append(calls, "ToValues")
				return nil, nil
			},
		}

		ctx := NewVMContext(vmCtxParams)
		ctx.deps = deps

		_, code, err := ctx.Send(newAddress(), "foo", types.ZeroAttoFIL, []interface{}{})

		assert.Error(t, err)
		assert.Equal(t, 1, int(code))
		assert.True(t, errors.ShouldRevert(err))
		assert.Equal(t, []string{"ToValues", "EncodeValues"}, calls)
	})

	t.Run("refuse to send a message with identical from/to", func(t *testing.T) {
		to := newAddress()

		msg := newMsg()
		msg.To = to

		var calls []string
		deps := &deps{
			EncodeValues: func(_ []*abi.Value) ([]byte, error) {
				calls = append(calls, "EncodeValues")
				return nil, nil
			},
			ToValues: func(_ []interface{}) ([]*abi.Value, error) {
				calls = append(calls, "ToValues")
				return nil, nil
			},
		}
		vmCtxParams.Message = msg
		ctx := NewVMContext(vmCtxParams)
		ctx.deps = deps

		_, code, err := ctx.Send(to, "foo", types.ZeroAttoFIL, []interface{}{})

		assert.Error(t, err)
		assert.Equal(t, 1, int(code))
		assert.True(t, errors.IsFault(err))
		assert.Equal(t, []string{"ToValues", "EncodeValues"}, calls)
	})

	t.Run("returns a fault error if unable to create or find a recipient actor", func(t *testing.T) {
		var calls []string
		deps := &deps{
			EncodeValues: func(_ []*abi.Value) ([]byte, error) {
				calls = append(calls, "EncodeValues")
				return nil, nil
			},
			GetOrCreateActor: func(_ context.Context, _ address.Address, _ func() (*actor.Actor, error)) (*actor.Actor, error) {

				calls = append(calls, "GetOrCreateActor")
				return nil, xerrors.New("error")
			},
			ToValues: func(_ []interface{}) ([]*abi.Value, error) {
				calls = append(calls, "ToValues")
				return nil, nil
			},
		}

		vmCtxParams.Message = newMsg()
		ctx := NewVMContext(vmCtxParams)
		ctx.deps = deps

		_, code, err := ctx.Send(newAddress(), "foo", types.ZeroAttoFIL, []interface{}{})

		assert.Error(t, err)
		assert.Equal(t, 1, int(code))
		assert.True(t, errors.IsFault(err))
		assert.Equal(t, []string{"ToValues", "EncodeValues", "GetOrCreateActor"}, calls)
	})

	t.Run("propagates any error returned from Send", func(t *testing.T) {
		expectedVMSendErr := xerrors.New("error")

		var calls []string
		deps := &deps{
			EncodeValues: func(_ []*abi.Value) ([]byte, error) {
				calls = append(calls, "EncodeValues")
				return nil, nil
			},
			GetOrCreateActor: func(_ context.Context, _ address.Address, f func() (*actor.Actor, error)) (*actor.Actor, error) {
				calls = append(calls, "GetOrCreateActor")
				return f()
			},
			Send: func(ctx context.Context, vmCtx *Context) ([][]byte, uint8, error) {
				calls = append(calls, "Send")
				return nil, 123, expectedVMSendErr
			},
			ToValues: func(_ []interface{}) ([]*abi.Value, error) {
				calls = append(calls, "ToValues")
				return nil, nil
			},
		}

		ctx := NewVMContext(vmCtxParams)
		ctx.deps = deps

		_, code, err := ctx.Send(newAddress(), "foo", types.ZeroAttoFIL, []interface{}{})

		assert.Error(t, err)
		assert.Equal(t, 123, int(code))
		assert.Equal(t, expectedVMSendErr, err)
		assert.Equal(t, []string{"ToValues", "EncodeValues", "GetOrCreateActor", "Send"}, calls)
	})

	t.Run("creates new actor from cid", func(t *testing.T) {
		ctx := context.Background()
		vmctx := NewVMContext(vmCtxParams)
		addr, err := vmctx.AddressForNewActor()

		require.NoError(t, err)

		params := &actor.FakeActorStorage{}
		err = vmctx.CreateNewActor(addr, fakeActorCid, params)
		require.NoError(t, err)

		act, err := tree.GetActor(ctx, addr)
		require.NoError(t, err)

		assert.Equal(t, fakeActorCid, act.Code)
		actorStorage := vms.NewStorage(addr, act)
		chunk, err := actorStorage.Get(act.Head)
		require.NoError(t, err)

		assert.True(t, len(chunk) > 0)
	})

}

func TestVMContextIsAccountActor(t *testing.T) {
	tf.UnitTest(t)

	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	vms := NewStorageMap(bs)

	accountActor, err := account.NewActor(types.NewAttoFILFromFIL(1000))
	require.NoError(t, err)
	vmCtxParams := NewContextParams{
		From:       accountActor,
		StorageMap: vms,
		GasTracker: NewGasTracker(),
	}

	ctx := NewVMContext(vmCtxParams)
	assert.True(t, ctx.IsFromAccountActor())

	nonAccountActor := actor.NewActor(types.NewCidForTestGetter()(), types.NewAttoFILFromFIL(1000))
	vmCtxParams.From = nonAccountActor
	ctx = NewVMContext(vmCtxParams)
	assert.False(t, ctx.IsFromAccountActor())
}
