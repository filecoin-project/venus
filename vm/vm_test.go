package vm

import (
	"context"
	"testing"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmRu7tiRnFk9mMPpVECQTBQJqXtmG132jJxA1w9A7TtpBz/go-ipfs-blockstore"
	"gx/ipfs/QmUadX5EcvrBmxAV9sE7wUWtWSqxns5K84qKJBixmcT1w9/go-datastore"
	xerrors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm/errors"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
)

func TestTransfer(t *testing.T) {
	actor1 := actor.NewActor(cid.Undef, types.NewAttoFILFromFIL(100))
	actor2 := actor.NewActor(cid.Undef, types.NewAttoFILFromFIL(50))
	actor3 := actor.NewActor(cid.Undef, nil)

	t.Run("success", func(t *testing.T) {
		assert := assert.New(t)

		assert.NoError(Transfer(actor1, actor2, types.NewAttoFILFromFIL(10)))
		assert.Equal(actor1.Balance, types.NewAttoFILFromFIL(90))
		assert.Equal(actor2.Balance, types.NewAttoFILFromFIL(60))

		assert.NoError(Transfer(actor1, actor3, types.NewAttoFILFromFIL(20)))
		assert.Equal(actor1.Balance, types.NewAttoFILFromFIL(70))
		assert.Equal(actor3.Balance, types.NewAttoFILFromFIL(20))
	})

	t.Run("fail", func(t *testing.T) {
		assert := assert.New(t)

		negval := types.NewAttoFILFromFIL(0).Sub(types.NewAttoFILFromFIL(1000))
		assert.EqualError(Transfer(actor2, actor3, types.NewAttoFILFromFIL(1000)), "not enough balance")
		assert.EqualError(Transfer(actor2, actor3, negval), "cannot transfer negative values")
	})
}

func TestSendErrorHandling(t *testing.T) {
	actor1 := actor.NewActor(types.SomeCid(), types.NewAttoFILFromFIL(100))
	actor2 := actor.NewActor(types.SomeCid(), types.NewAttoFILFromFIL(50))
	newMsg := types.NewMessageForTestGetter()

	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	vms := NewStorageMap(bs)

	t.Run("returns exit code 1 and an unwrapped error if we fail to transfer value from one actor to another", func(t *testing.T) {
		assert := assert.New(t)

		transferErr := xerrors.New("error")

		msg := newMsg()
		msg.Value = types.NewAttoFILFromFIL(1) // exact value doesn't matter - needs to be non-nil

		deps := sendDeps{
			transfer: func(_ *actor.Actor, _ *actor.Actor, _ *types.AttoFIL) error {
				return transferErr
			},
		}

		tree := state.NewCachedStateTree(&state.MockStateTree{NoMocks: true})
		vmCtxParams := NewContextParams{
			From:        actor1,
			To:          actor2,
			Message:     msg,
			State:       tree,
			StorageMap:  vms,
			GasTracker:  NewGasTracker(),
			BlockHeight: types.NewBlockHeight(0),
		}
		vmCtx := NewVMContext(vmCtxParams)
		_, code, sendErr := send(context.Background(), deps, vmCtx)

		assert.Error(sendErr)
		assert.Equal(1, int(code))
		assert.Equal(transferErr, sendErr)
	})

	t.Run("returns right exit code and a revert error if we can't load the recipient actor's code", func(t *testing.T) {
		assert := assert.New(t)

		msg := newMsg()
		msg.Value = nil // such that we don't transfer

		deps := sendDeps{}

		tree := state.NewCachedStateTree(&state.MockStateTree{NoMocks: true, BuiltinActors: map[cid.Cid]exec.ExecutableActor{}})
		vmCtxParams := NewContextParams{
			From:        actor1,
			To:          actor2,
			Message:     msg,
			State:       tree,
			StorageMap:  vms,
			GasTracker:  NewGasTracker(),
			BlockHeight: types.NewBlockHeight(0),
		}
		vmCtx := NewVMContext(vmCtxParams)
		_, code, sendErr := send(context.Background(), deps, vmCtx)

		assert.Error(sendErr)
		assert.Equal(errors.ErrNoActorCode, int(code))
		assert.True(errors.ShouldRevert(sendErr))
	})

	t.Run("returns exit code 1 and a revert error if code doesn't export a matching method", func(t *testing.T) {
		assert := assert.New(t)

		msg := newMsg()
		msg.Value = nil // such that we don't transfer
		msg.Method = "bar"

		assert.False(actor.FakeActorExports.Has(msg.Method))

		deps := sendDeps{}

		tree := state.NewCachedStateTree(&state.MockStateTree{NoMocks: true, BuiltinActors: map[cid.Cid]exec.ExecutableActor{
			actor2.Code: &actor.FakeActor{},
		}})

		vmCtxParams := NewContextParams{
			From:        actor1,
			To:          actor2,
			Message:     msg,
			State:       tree,
			StorageMap:  vms,
			GasTracker:  NewGasTracker(),
			BlockHeight: types.NewBlockHeight(0),
		}
		vmCtx := NewVMContext(vmCtxParams)
		_, code, sendErr := send(context.Background(), deps, vmCtx)

		assert.Error(sendErr)
		assert.Equal(1, int(code))
		assert.True(errors.ShouldRevert(sendErr))
	})
}
