package vm

import (
	"context"
	"testing"

	cbor "gx/ipfs/QmRiRJhn427YVuufBEHofLreKWNw7P7BWNq86Sb9kzqdbd/go-ipld-cbor"
	xerrors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmcYBp5EDnJKfVN63F71rDTksvEf1cfijwCTWtw6bPG58T/go-hamt-ipld"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm/errors"
	"github.com/stretchr/testify/require"
)

func TestVMContextStorage(t *testing.T) {
	assert := assert.New(t)
	addrGetter := types.NewAddressForTestGetter()
	ctx := context.Background()

	cst := hamt.NewCborStore()
	st := state.NewEmptyStateTree(cst)
	cstate := state.NewCachedStateTree(st)

	r := repo.NewInMemoryRepo()
	ds := r.Datastore()
	vms := NewStorageMap(ds)

	toActor, err := account.NewActor(nil)
	assert.NoError(err)
	toAddr := addrGetter()

	assert.NoError(st.SetActor(ctx, toAddr, toActor))
	msg := types.NewMessage(addrGetter(), toAddr, 0, nil, "hello", nil)

	to, err := cstate.GetActor(ctx, toAddr)
	assert.NoError(err)
	vmCtx := NewVMContext(nil, to, msg, cstate, vms, types.NewBlockHeight(0))

	node, err := cbor.WrapObject([]byte("hello"), types.DefaultHashFunction, -1)
	assert.NoError(err)

	assert.NoError(vmCtx.WriteStorage(node.RawData()))
	assert.NoError(cstate.Commit(ctx))

	// make sure we can read it back
	toActorBack, err := st.GetActor(ctx, toAddr)
	assert.NoError(err)

	storage, err := NewVMContext(nil, toActorBack, msg, cstate, vms, types.NewBlockHeight(0)).ReadStorage()
	assert.NoError(err)
	assert.Equal(storage, node.RawData())
}

func TestVMContextSendFailures(t *testing.T) {
	actor1 := types.NewActor(nil, types.NewAttoFILFromFIL(100))
	actor2 := types.NewActor(nil, types.NewAttoFILFromFIL(50))
	newMsg := types.NewMessageForTestGetter()
	newAddress := types.NewAddressForTestGetter()

	tree := state.NewCachedStateTree(&state.MockStateTree{})
	r := repo.NewInMemoryRepo()
	ds := r.Datastore()
	vms := NewStorageMap(ds)

	t.Run("failure to convert to ABI values results in fault error", func(t *testing.T) {
		assert := assert.New(t)

		var calls []string
		deps := &deps{
			ToValues: func(_ []interface{}) ([]*abi.Value, error) {
				calls = append(calls, "ToValues")
				return nil, xerrors.New("error")
			},
		}

		ctx := NewVMContext(actor1, actor2, newMsg(), tree, vms, types.NewBlockHeight(0))
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

		ctx := NewVMContext(actor1, actor2, newMsg(), tree, vms, types.NewBlockHeight(0))
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

		ctx := NewVMContext(actor1, actor2, msg, tree, vms, types.NewBlockHeight(0))
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

		ctx := NewVMContext(actor1, actor2, newMsg(), tree, vms, types.NewBlockHeight(0))
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
			Send: func(ctx context.Context, vmCtx *Context) ([][]byte, uint8, error) {
				calls = append(calls, "Send")
				return nil, 123, expectedVMSendErr
			},
			ToValues: func(_ []interface{}) ([]*abi.Value, error) {
				calls = append(calls, "ToValues")
				return nil, nil
			},
		}

		ctx := NewVMContext(actor1, actor2, newMsg(), tree, vms, types.NewBlockHeight(0))
		ctx.deps = deps

		_, code, err := ctx.Send(newAddress(), "foo", nil, []interface{}{})

		assert.Error(err)
		assert.Equal(123, int(code))
		assert.Equal(expectedVMSendErr, err)
		assert.Equal([]string{"ToValues", "EncodeValues", "GetOrCreateActor", "Send"}, calls)
	})
}

func TestVMContextIsAccountActor(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	r := repo.NewInMemoryRepo()
	ds := r.Datastore()
	vms := NewStorageMap(ds)

	accountActor, err := account.NewActor(types.NewAttoFILFromFIL(1000))
	require.NoError(err)
	ctx := NewVMContext(accountActor, nil, nil, nil, vms, nil)
	assert.True(ctx.IsFromAccountActor())

	nonAccountActor := types.NewActor(types.NewCidForTestGetter()(), types.NewAttoFILFromFIL(1000))
	ctx = NewVMContext(nonAccountActor, nil, nil, nil, vms, nil)
	assert.False(ctx.IsFromAccountActor())
}
