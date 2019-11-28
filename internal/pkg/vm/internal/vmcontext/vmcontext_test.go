package vmcontext

import (
	"context"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	xerrors "github.com/pkg/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/gastracker"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/storagemap"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

func TestVMContextStorage(t *testing.T) {
	tf.UnitTest(t)

	addrGetter := address.NewForTestGetter()
	ctx := context.Background()

	cst := hamt.NewCborStore()
	st := state.NewTree(cst)
	cstate := state.NewCachedTree(st)

	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	vms := storagemap.NewStorageMap(bs)

	toActor, err := account.NewActor(types.ZeroAttoFIL)
	assert.NoError(t, err)
	toAddr := addrGetter()

	helloID := types.MethodID(82723)

	assert.NoError(t, st.SetActor(ctx, toAddr, toActor))
	msg := types.NewUnsignedMessage(addrGetter(), toAddr, 0, types.ZeroAttoFIL, helloID, nil)

	to, err := cstate.GetActor(ctx, toAddr)
	assert.NoError(t, err)
	vmCtxParams := NewContextParams{
		From:        nil,
		To:          to,
		Message:     msg,
		State:       cstate,
		StorageMap:  vms,
		GasTracker:  gastracker.NewGasTracker(),
		BlockHeight: types.NewBlockHeight(0),
	}
	vmCtx := NewVMContext(vmCtxParams)

	node, err := cbor.WrapObject(helloID, types.DefaultHashFunction, -1)
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
		BuiltinActors: map[cid.Cid]dispatch.ExecutableActor{},
	}
	fakeActorCid := types.NewCidForTestGetter()()
	mockStateTree.BuiltinActors[fakeActorCid] = &actor.FakeActor{}
	tree := state.NewCachedTree(&mockStateTree)
	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	vms := storagemap.NewStorageMap(bs)

	msg := newMsg()

	vmCtxParams := func() NewContextParams {
		return NewContextParams{
			From:        actor1,
			To:          actor2,
			Message:     msg,
			OriginMsg:   msg,
			State:       tree,
			StorageMap:  vms,
			GasTracker:  gastracker.NewGasTracker(),
			BlockHeight: types.NewBlockHeight(0),
			Actors:      &mockStateTree,
		}
	}

	fooID := types.MethodID(8272)

	t.Run("failure to convert to ABI values results in fault error", func(t *testing.T) {
		var calls []string
		deps := &deps{
			ToValues: func(_ []interface{}) ([]*abi.Value, error) {
				calls = append(calls, "ToValues")
				return nil, xerrors.New("error")
			},
		}

		ctx := NewVMContext(vmCtxParams())
		ctx.deps = deps

		_, code, err := ctx.Send(newAddress(), fooID, types.ZeroAttoFIL, []interface{}{})

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

		ctx := NewVMContext(vmCtxParams())
		ctx.deps = deps

		_, code, err := ctx.Send(newAddress(), fooID, types.ZeroAttoFIL, []interface{}{})

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
		params := vmCtxParams()
		params.Message = msg
		ctx := NewVMContext(params)
		ctx.deps = deps
		ctx.toAddr = msg.To

		_, code, err := ctx.Send(to, fooID, types.ZeroAttoFIL, []interface{}{})

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

		params := vmCtxParams()
		params.Message = newMsg()
		ctx := NewVMContext(params)
		ctx.deps = deps

		_, code, err := ctx.Send(newAddress(), fooID, types.ZeroAttoFIL, []interface{}{})

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
			Send: func(ctx context.Context, vmCtx ExtendedRuntime) ([][]byte, uint8, error) {
				calls = append(calls, "Send")
				return nil, 123, expectedVMSendErr
			},
			ToValues: func(_ []interface{}) ([]*abi.Value, error) {
				calls = append(calls, "ToValues")
				return nil, nil
			},
		}

		ctx := NewVMContext(vmCtxParams())
		ctx.deps = deps

		_, code, err := ctx.Send(newAddress(), fooID, types.ZeroAttoFIL, []interface{}{})

		assert.Error(t, err)
		assert.Equal(t, 123, int(code))
		assert.Equal(t, expectedVMSendErr, err)
		assert.Equal(t, []string{"ToValues", "EncodeValues", "GetOrCreateActor", "Send"}, calls)
	})

	t.Run("AddressForNewActor uses origin message", func(t *testing.T) {
		vmctx := NewVMContext(vmCtxParams())
		addr1, err := vmctx.LegacyAddressForNewActor()
		require.NoError(t, err)

		assert.Equal(t, addr1.Protocol(), address.Actor)

		// vmctx with same origin message produces same addr
		addr2, err := NewVMContext(vmCtxParams()).LegacyAddressForNewActor()
		require.NoError(t, err)
		assert.Equal(t, addr2, addr1)

		// vmctx with different origin message produces different addr
		params := vmCtxParams()
		params.OriginMsg = newMsg()
		params.OriginMsg.From = newAddress()
		params.OriginMsg.CallSeqNum = 42

		addr3, err := NewVMContext(params).LegacyAddressForNewActor()
		require.NoError(t, err)
		assert.NotEqual(t, addr3, addr1)
	})

	t.Run("creates new actor from cid", func(t *testing.T) {
		ctx := context.Background()
		vmctx := NewVMContext(vmCtxParams())
		addr, err := vmctx.LegacyAddressForNewActor()

		require.NoError(t, err)

		err = vmctx.LegacyCreateNewActor(addr, fakeActorCid)
		require.NoError(t, err)

		act, err := tree.GetActor(ctx, addr)
		require.NoError(t, err)

		assert.Equal(t, fakeActorCid, act.Code)
	})

}

func TestSendErrorHandling(t *testing.T) {
	tf.UnitTest(t)
	actor1 := actor.NewActor(types.CidFromString(t, "somecid"), types.NewAttoFILFromFIL(100))
	actor2 := actor.NewActor(types.CidFromString(t, "somecid"), types.NewAttoFILFromFIL(50))
	newMsg := types.NewMessageForTestGetter()

	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	vms := storagemap.NewStorageMap(bs)

	t.Run("returns exit code 1 and an unwrapped error if we fail to transfer value from one actor to another", func(t *testing.T) {
		transferErr := xerrors.New("error")

		msg := newMsg()
		msg.Value = types.NewAttoFILFromFIL(1) // exact value doesn't matter - needs to be non-nil

		transfer := func(_ *actor.Actor, _ *actor.Actor, _ types.AttoFIL) error {
			return transferErr
		}

		tree := state.NewCachedTree(&state.MockStateTree{NoMocks: true})
		vmCtxParams := NewContextParams{
			From:        actor1,
			To:          actor2,
			Message:     msg,
			State:       tree,
			StorageMap:  vms,
			GasTracker:  gastracker.NewGasTracker(),
			BlockHeight: types.NewBlockHeight(0),
		}
		vmCtx := NewVMContext(vmCtxParams)
		_, code, sendErr := send(context.Background(), transfer, vmCtx)

		assert.Error(t, sendErr)
		assert.Equal(t, 1, int(code))
		assert.Equal(t, transferErr, sendErr)
	})

	t.Run("returns right exit code and a revert error if we can't load the recipient actor's code", func(t *testing.T) {
		msg := newMsg()
		msg.Value = types.ZeroAttoFIL // such that we don't transfer

		stateTree := &state.MockStateTree{NoMocks: true, BuiltinActors: map[cid.Cid]dispatch.ExecutableActor{}}
		tree := state.NewCachedTree(stateTree)
		vmCtxParams := NewContextParams{
			From:        actor1,
			To:          actor2,
			Message:     msg,
			State:       tree,
			StorageMap:  vms,
			GasTracker:  gastracker.NewGasTracker(),
			BlockHeight: types.NewBlockHeight(0),
			Actors:      stateTree,
		}
		vmCtx := NewVMContext(vmCtxParams)
		_, code, sendErr := send(context.Background(), Transfer, vmCtx)

		assert.Error(t, sendErr)
		assert.Equal(t, errors.ErrNoActorCode, int(code))
		assert.True(t, errors.ShouldRevert(sendErr))
	})

	t.Run("returns exit code 1 and a revert error if code doesn't export a matching method", func(t *testing.T) {
		msg := newMsg()
		msg.Value = types.ZeroAttoFIL // such that we don't transfer
		msg.Method = types.MethodID(125124)

		stateTree := &state.MockStateTree{NoMocks: true, BuiltinActors: map[cid.Cid]dispatch.ExecutableActor{
			actor2.Code: &actor.FakeActor{},
		}}
		tree := state.NewCachedTree(stateTree)

		vmCtxParams := NewContextParams{
			From:        actor1,
			To:          actor2,
			Message:     msg,
			State:       tree,
			StorageMap:  vms,
			GasTracker:  gastracker.NewGasTracker(),
			BlockHeight: types.NewBlockHeight(0),
			Actors:      stateTree,
		}
		vmCtx := NewVMContext(vmCtxParams)
		_, code, sendErr := send(context.Background(), Transfer, vmCtx)

		assert.Error(t, sendErr)
		assert.Equal(t, 1, int(code))
		assert.True(t, errors.ShouldRevert(sendErr))
	})
}

func TestTransfer(t *testing.T) {
	tf.UnitTest(t)

	actor1 := actor.NewActor(cid.Undef, types.NewAttoFILFromFIL(100))
	actor2 := actor.NewActor(cid.Undef, types.NewAttoFILFromFIL(50))
	actor3 := actor.NewActor(cid.Undef, types.ZeroAttoFIL)

	t.Run("success", func(t *testing.T) {
		assert.NoError(t, Transfer(actor1, actor2, types.NewAttoFILFromFIL(10)))
		assert.Equal(t, actor1.Balance, types.NewAttoFILFromFIL(90))
		assert.Equal(t, actor2.Balance, types.NewAttoFILFromFIL(60))

		assert.NoError(t, Transfer(actor1, actor3, types.NewAttoFILFromFIL(20)))
		assert.Equal(t, actor1.Balance, types.NewAttoFILFromFIL(70))
		assert.Equal(t, actor3.Balance, types.NewAttoFILFromFIL(20))
	})

	t.Run("fail", func(t *testing.T) {
		negval := types.NewAttoFILFromFIL(0).Sub(types.NewAttoFILFromFIL(1000))
		assert.EqualError(t, Transfer(actor2, actor3, types.NewAttoFILFromFIL(1000)), "not enough balance")
		assert.EqualError(t, Transfer(actor2, actor3, negval), "cannot transfer negative values")
	})
}
