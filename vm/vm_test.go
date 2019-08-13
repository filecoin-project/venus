package vm

import (
	"context"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-ipfs-blockstore"
	xerrors "github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm/errors"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
)

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

func TestSendErrorHandling(t *testing.T) {
	tf.UnitTest(t)
	actor1 := actor.NewActor(types.CidFromString(t, "somecid"), types.NewAttoFILFromFIL(100))
	actor2 := actor.NewActor(types.CidFromString(t, "somecid"), types.NewAttoFILFromFIL(50))
	newMsg := types.NewMessageForTestGetter()

	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	vms := NewStorageMap(bs)

	t.Run("returns exit code 1 and an unwrapped error if we fail to transfer value from one actor to another", func(t *testing.T) {
		transferErr := xerrors.New("error")

		msg := newMsg()
		msg.Value = types.NewAttoFILFromFIL(1) // exact value doesn't matter - needs to be non-nil

		deps := sendDeps{
			transfer: func(_ *actor.Actor, _ *actor.Actor, _ types.AttoFIL) error {
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

		assert.Error(t, sendErr)
		assert.Equal(t, 1, int(code))
		assert.Equal(t, transferErr, sendErr)
	})

	t.Run("returns right exit code and a revert error if we can't load the recipient actor's code", func(t *testing.T) {
		msg := newMsg()
		msg.Value = types.ZeroAttoFIL // such that we don't transfer

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

		assert.Error(t, sendErr)
		assert.Equal(t, errors.ErrNoActorCode, int(code))
		assert.True(t, errors.ShouldRevert(sendErr))
	})

	t.Run("returns exit code 1 and a revert error if code doesn't export a matching method", func(t *testing.T) {
		msg := newMsg()
		msg.Value = types.ZeroAttoFIL // such that we don't transfer
		msg.Method = "bar"

		assert.False(t, actor.FakeActorExports.Has(msg.Method))

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

		assert.Error(t, sendErr)
		assert.Equal(t, 1, int(code))
		assert.True(t, errors.ShouldRevert(sendErr))
	})
}
