package vm

import (
	"context"
	"testing"

	"gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore"
	xerrors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmcmpX42gtDv1fz24kau4wjS9hfwWj5VexWBKgGnWzsyag/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm/errors"

	"github.com/stretchr/testify/assert"
)

func TestTransfer(t *testing.T) {
	actor1 := types.NewActor(nil, types.NewAttoFILFromFIL(100))
	actor2 := types.NewActor(nil, types.NewAttoFILFromFIL(50))
	actor3 := types.NewActor(nil, nil)

	t.Run("success", func(t *testing.T) {
		assert := assert.New(t)

		assert.NoError(transfer(actor1, actor2, types.NewAttoFILFromFIL(10)))
		assert.Equal(actor1.Balance, types.NewAttoFILFromFIL(90))
		assert.Equal(actor2.Balance, types.NewAttoFILFromFIL(60))

		assert.NoError(transfer(actor1, actor3, types.NewAttoFILFromFIL(20)))
		assert.Equal(actor1.Balance, types.NewAttoFILFromFIL(70))
		assert.Equal(actor3.Balance, types.NewAttoFILFromFIL(20))
	})

	t.Run("fail", func(t *testing.T) {
		assert := assert.New(t)

		negval := types.NewAttoFILFromFIL(0).Sub(types.NewAttoFILFromFIL(1000))
		assert.EqualError(transfer(actor2, actor3, types.NewAttoFILFromFIL(1000)), "not enough balance")
		assert.EqualError(transfer(actor2, actor3, negval), "cannot transfer negative values")
	})
}

func TestSendErrorHandling(t *testing.T) {
	actor1 := types.NewActor(types.SomeCid(), types.NewAttoFILFromFIL(100))
	actor2 := types.NewActor(types.SomeCid(), types.NewAttoFILFromFIL(50))
	newMsg := types.NewMessageForTestGetter()

	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	vms := NewStorageMap(bs)

	t.Run("returns exit code 1 and an unwrapped error if we fail to transfer value from one actor to another", func(t *testing.T) {
		assert := assert.New(t)

		transferErr := xerrors.New("error")

		msg := newMsg()
		msg.Value = types.NewAttoFILFromFIL(1) // exact value doesn't matter - needs to be non-nil

		deps := sendDeps{
			transfer: func(_ *types.Actor, _ *types.Actor, _ *types.AttoFIL) error {
				return transferErr
			},
		}

		tree := state.NewCachedStateTree(&state.MockStateTree{NoMocks: true})
		vmCtx := NewVMContext(actor1, actor2, msg, tree, vms, types.NewBlockHeight(0))
		_, code, sendErr := send(context.Background(), deps, vmCtx)

		assert.Error(sendErr)
		assert.Equal(1, int(code))
		assert.Equal(transferErr, sendErr)
	})

	t.Run("returns exit code 1 and a fault error if we can't load the recipient actor's code", func(t *testing.T) {
		assert := assert.New(t)

		msg := newMsg()
		msg.Value = nil // such that we don't transfer

		deps := sendDeps{}

		tree := state.NewCachedStateTree(&state.MockStateTree{NoMocks: true, BuiltinActors: map[string]exec.ExecutableActor{}})
		vmCtx := NewVMContext(actor1, actor2, msg, tree, vms, types.NewBlockHeight(0))
		_, code, sendErr := send(context.Background(), deps, vmCtx)

		assert.Error(sendErr)
		assert.Equal(1, int(code))
		assert.True(errors.IsFault(sendErr))
	})

	t.Run("returns exit code 1 and a revert error if code doesn't export a matching method", func(t *testing.T) {
		assert := assert.New(t)

		msg := newMsg()
		msg.Value = nil // such that we don't transfer
		msg.Method = "bar"

		assert.False(actor.FakeActorExports.Has(msg.Method))

		deps := sendDeps{}

		tree := state.NewCachedStateTree(&state.MockStateTree{NoMocks: true, BuiltinActors: map[string]exec.ExecutableActor{
			actor2.Code.KeyString(): &actor.FakeActor{},
		}})
		vmCtx := NewVMContext(actor1, actor2, msg, tree, vms, types.NewBlockHeight(0))
		_, code, sendErr := send(context.Background(), deps, vmCtx)

		assert.Error(sendErr)
		assert.Equal(1, int(code))
		assert.True(errors.ShouldRevert(sendErr))
	})
}
