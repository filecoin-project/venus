package vm

import (
	"context"
	"testing"

	xerrors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmdtiofXbibTe6Day9ii5zjBZpSRm8vhfoerrNuY3sAQ7e/go-hamt-ipld"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm/errors"
)

func TestVMContextStorage(t *testing.T) {
	assert := assert.New(t)
	addrGetter := types.NewAddressForTestGetter()
	ctx := context.Background()

	cst := hamt.NewCborStore()
	state := state.NewEmptyStateTree(cst)

	toActor, err := account.NewActor(nil)
	assert.NoError(err)
	toAddr := addrGetter()

	assert.NoError(state.SetActor(ctx, toAddr, toActor))

	msg := types.NewMessage(addrGetter(), toAddr, 0, nil, "hello", nil)

	vmCtx := NewVMContext(nil, toActor, msg, state)

	assert.NoError(vmCtx.WriteStorage([]byte("hello")))

	// make sure we can read it back
	toActorBack, err := state.GetActor(ctx, toAddr)
	assert.NoError(err)

	storage := NewVMContext(nil, toActorBack, msg, state).ReadStorage()
	assert.Equal(storage, []byte("hello"))
}

func TestVMContextSendFailures(t *testing.T) {
	actor1 := types.NewActor(nil, types.NewTokenAmount(100))
	actor2 := types.NewActor(nil, types.NewTokenAmount(50))
	newMsg := types.NewMessageForTestGetter()
	newAddress := types.NewAddressForTestGetter()

	t.Run("failure to convert to ABI values results in fault error", func(t *testing.T) {
		assert := assert.New(t)

		var calls []string
		deps := &deps{
			ToValues: func(_ []interface{}) ([]*abi.Value, error) {
				calls = append(calls, "ToValues")
				return nil, xerrors.New("error")
			},
		}

		ctx := NewVMContext(actor1, actor2, newMsg(), &state.MockStateTree{})
		ctx.deps = deps

		_, code, err := ctx.Send(newAddress(), "foo", nil, []interface{}{})

		assert.Error(err)
		assert.Equal(1, int(code))
		assert.True(errors.IsFault(err))
		assert.Equal([]string{"ToValues"}, calls)
	})

	t.Run("failure to encode ABI values to byte slice results in revert error", func(t *testing.T) {
		assert := assert.New(t)

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

		ctx := NewVMContext(actor1, actor2, newMsg(), &state.MockStateTree{})
		ctx.deps = deps

		_, code, err := ctx.Send(newAddress(), "foo", nil, []interface{}{})

		assert.Error(err)
		assert.Equal(1, int(code))
		assert.True(errors.ShouldRevert(err))
		assert.Equal([]string{"ToValues", "EncodeValues"}, calls)
	})

	t.Run("refuse to send a message with identical from/to", func(t *testing.T) {
		assert := assert.New(t)

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

		ctx := NewVMContext(actor1, actor2, msg, &state.MockStateTree{})
		ctx.deps = deps

		_, code, err := ctx.Send(to, "foo", nil, []interface{}{})

		assert.Error(err)
		assert.Equal(1, int(code))
		assert.True(errors.IsFault(err))
		assert.Equal([]string{"ToValues", "EncodeValues"}, calls)
	})

	t.Run("returns a fault error if unable to create or find a recipient actor", func(t *testing.T) {
		assert := assert.New(t)

		var calls []string
		deps := &deps{
			EncodeValues: func(_ []*abi.Value) ([]byte, error) {
				calls = append(calls, "EncodeValues")
				return nil, nil
			},
			GetOrCreateActor: func(_ context.Context, _ types.Address, _ func() (*types.Actor, error)) (*types.Actor, error) {
				calls = append(calls, "GetOrCreateActor")
				return nil, xerrors.New("error")
			},
			ToValues: func(_ []interface{}) ([]*abi.Value, error) {
				calls = append(calls, "ToValues")
				return nil, nil
			},
		}

		ctx := NewVMContext(actor1, actor2, newMsg(), &state.MockStateTree{})
		ctx.deps = deps

		_, code, err := ctx.Send(newAddress(), "foo", nil, []interface{}{})

		assert.Error(err)
		assert.Equal(1, int(code))
		assert.True(errors.IsFault(err))
		assert.Equal([]string{"ToValues", "EncodeValues", "GetOrCreateActor"}, calls)
	})

	t.Run("propagates any error returned from Send", func(t *testing.T) {
		assert := assert.New(t)

		expectedVMSendErr := xerrors.New("error")

		var calls []string
		deps := &deps{
			EncodeValues: func(_ []*abi.Value) ([]byte, error) {
				calls = append(calls, "EncodeValues")
				return nil, nil
			},
			GetOrCreateActor: func(_ context.Context, _ types.Address, f func() (*types.Actor, error)) (*types.Actor, error) {
				calls = append(calls, "GetOrCreateActor")
				return f()
			},
			Send: func(ctx context.Context, from, to *types.Actor, msg *types.Message, st state.Tree) ([]byte, uint8, error) {
				calls = append(calls, "Send")
				return nil, 123, expectedVMSendErr
			},
			SetActor: func(_ context.Context, _ types.Address, _ *types.Actor) error {
				calls = append(calls, "SetActor")
				return nil
			},
			ToValues: func(_ []interface{}) ([]*abi.Value, error) {
				calls = append(calls, "ToValues")
				return nil, nil
			},
		}

		ctx := NewVMContext(actor1, actor2, newMsg(), &state.MockStateTree{})
		ctx.deps = deps

		_, code, err := ctx.Send(newAddress(), "foo", nil, []interface{}{})

		assert.Error(err)
		assert.Equal(123, int(code))
		assert.Equal(expectedVMSendErr, err)
		assert.Equal([]string{"ToValues", "EncodeValues", "GetOrCreateActor", "Send"}, calls)
	})
}
