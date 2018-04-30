package core

import (
	"context"
	"testing"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
)

func TestTransfer(t *testing.T) {
	actor1 := types.NewActor(nil, types.NewTokenAmount(100))
	actor2 := types.NewActor(nil, types.NewTokenAmount(50))
	actor3 := types.NewActor(nil, nil)

	t.Run("success", func(t *testing.T) {
		assert := assert.New(t)

		assert.NoError(transfer(actor1, actor2, types.NewTokenAmount(10)))
		assert.Equal(actor1.Balance, types.NewTokenAmount(90))
		assert.Equal(actor2.Balance, types.NewTokenAmount(60))

		assert.NoError(transfer(actor1, actor3, types.NewTokenAmount(20)))
		assert.Equal(actor1.Balance, types.NewTokenAmount(70))
		assert.Equal(actor3.Balance, types.NewTokenAmount(20))
	})

	t.Run("fail", func(t *testing.T) {
		assert := assert.New(t)

		negval := types.NewTokenAmount(0).Sub(types.NewTokenAmount(1000))
		assert.EqualError(transfer(actor2, actor3, types.NewTokenAmount(1000)), "not enough balance")
		assert.EqualError(transfer(actor2, actor3, negval), "cannot transfer negative values")
	})
}

func TestSendErrorHandling(t *testing.T) {
	actor1 := types.NewActor(types.SomeCid(), types.NewTokenAmount(100))
	actor2 := types.NewActor(types.SomeCid(), types.NewTokenAmount(50))
	newMsg := types.NewMessageForTestGetter()

	t.Run("returns exit code 1 and an unwrapped error if we fail to transfer value from one actor to another", func(t *testing.T) {
		assert := assert.New(t)

		transferErr := errors.New("error")

		msg := newMsg()
		msg.Value = types.NewTokenAmount(1) // exact value doesn't matter - needs to be non-nil

		deps := sendDeps{
			transfer: func(_ *types.Actor, _ *types.Actor, _ *types.TokenAmount) error {
				return transferErr
			},
		}

		_, code, sendErr := send(context.Background(), deps, actor1, actor2, msg, &state.MockStateTree{NoMocks: true})

		assert.Error(sendErr)
		assert.Equal(1, int(code))
		assert.Equal(transferErr, sendErr)
	})

	t.Run("returns exit code 1 and a fault error if we can't load the recipient actor's code", func(t *testing.T) {
		assert := assert.New(t)

		msg := newMsg()
		msg.Value = nil // such that we don't transfer

		deps := sendDeps{}

		_, code, sendErr := send(context.Background(), deps, actor1, actor2, msg, &state.MockStateTree{NoMocks: true, BuiltinActors: map[string]exec.ExecutableActor{}})

		assert.Error(sendErr)
		assert.Equal(1, int(code))
		assert.True(IsFault(sendErr))
	})

	t.Run("returns exit code 1 and a revert error if code doesn't export a matching method", func(t *testing.T) {
		assert := assert.New(t)

		msg := newMsg()
		msg.Value = nil // such that we don't transfer
		msg.Method = "bar"

		assert.False(fakeActorExports.Has(msg.Method))

		deps := sendDeps{}

		_, code, sendErr := send(context.Background(), deps, actor1, actor2, msg, &state.MockStateTree{NoMocks: true, BuiltinActors: map[string]exec.ExecutableActor{
			actor2.Code.KeyString(): &FakeActor{},
		}})

		assert.Error(sendErr)
		assert.Equal(1, int(code))
		assert.True(shouldRevert(sendErr))
	})
}
