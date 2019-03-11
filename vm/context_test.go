package vm

import (
	"context"
	"strconv"
	"testing"

	"gx/ipfs/QmNf3wujpV2Y7Lnj2hy2UrmuX8bhMDStRHbnSLh7Ypf36h/go-hamt-ipld"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmRu7tiRnFk9mMPpVECQTBQJqXtmG132jJxA1w9A7TtpBz/go-ipfs-blockstore"
	"gx/ipfs/QmUadX5EcvrBmxAV9sE7wUWtWSqxns5K84qKJBixmcT1w9/go-datastore"
	xerrors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/sampling"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm/errors"
)

func TestVMContextStorage(t *testing.T) {
	assert := assert.New(t)
	addrGetter := address.NewForTestGetter()
	ctx := context.Background()

	cst := hamt.NewCborStore()
	st := state.NewEmptyStateTree(cst)
	cstate := state.NewCachedStateTree(st)

	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	vms := NewStorageMap(bs)

	toActor, err := account.NewActor(nil)
	assert.NoError(err)
	toAddr := addrGetter()

	assert.NoError(st.SetActor(ctx, toAddr, toActor))
	msg := types.NewMessage(addrGetter(), toAddr, 0, nil, "hello", nil)

	to, err := cstate.GetActor(ctx, toAddr)
	assert.NoError(err)
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
	assert.NoError(err)

	assert.NoError(vmCtx.WriteStorage(node.RawData()))
	assert.NoError(cstate.Commit(ctx))

	// make sure we can read it back
	toActorBack, err := st.GetActor(ctx, toAddr)
	assert.NoError(err)
	vmCtxParams.To = toActorBack
	storage, err := NewVMContext(vmCtxParams).ReadStorage()
	assert.NoError(err)
	assert.Equal(storage, node.RawData())
}

func TestVMContextSendFailures(t *testing.T) {
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
		assert := assert.New(t)

		var calls []string
		deps := &deps{
			ToValues: func(_ []interface{}) ([]*abi.Value, error) {
				calls = append(calls, "ToValues")
				return nil, xerrors.New("error")
			},
		}

		ctx := NewVMContext(vmCtxParams)
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

		ctx := NewVMContext(vmCtxParams)
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
		vmCtxParams.Message = msg
		ctx := NewVMContext(vmCtxParams)
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

		_, code, err := ctx.Send(newAddress(), "foo", nil, []interface{}{})

		assert.Error(err)
		assert.Equal(123, int(code))
		assert.Equal(expectedVMSendErr, err)
		assert.Equal([]string{"ToValues", "EncodeValues", "GetOrCreateActor", "Send"}, calls)
	})

	t.Run("creates new actor from cid", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		ctx := context.Background()
		vmctx := NewVMContext(vmCtxParams)
		addr, err := vmctx.AddressForNewActor()

		require.NoError(err)

		params := &actor.FakeActorStorage{}
		err = vmctx.CreateNewActor(addr, fakeActorCid, params)
		require.NoError(err)

		act, err := tree.GetActor(ctx, addr)
		require.NoError(err)

		assert.Equal(fakeActorCid, act.Code)
		actorStorage := vms.NewStorage(addr, act)
		chunk, err := actorStorage.Get(act.Head)
		require.NoError(err)

		assert.True(len(chunk) > 0)
	})

}

func TestVMContextIsAccountActor(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	vms := NewStorageMap(bs)

	accountActor, err := account.NewActor(types.NewAttoFILFromFIL(1000))
	require.NoError(err)
	vmCtxParams := NewContextParams{
		From:       accountActor,
		StorageMap: vms,
		GasTracker: NewGasTracker(),
	}

	ctx := NewVMContext(vmCtxParams)
	assert.True(ctx.IsFromAccountActor())

	nonAccountActor := actor.NewActor(types.NewCidForTestGetter()(), types.NewAttoFILFromFIL(1000))
	vmCtxParams.From = nonAccountActor
	ctx = NewVMContext(vmCtxParams)
	assert.False(ctx.IsFromAccountActor())
}

func TestVMContextRand(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	var tipSetsDescBlockHeight []types.TipSet
	// setup ancestor chain
	head := types.NewBlockForTest(nil, uint64(0))
	head.Ticket = []byte(strconv.Itoa(0))
	for i := 0; i < 20; i++ {
		tipSetsDescBlockHeight = append([]types.TipSet{types.RequireNewTipSet(require, head)}, tipSetsDescBlockHeight...)
		newBlock := types.NewBlockForTest(head, uint64(0))
		newBlock.Ticket = []byte(strconv.Itoa(i + 1))
		head = newBlock
	}
	tipSetsDescBlockHeight = append([]types.TipSet{types.RequireNewTipSet(require, head)}, tipSetsDescBlockHeight...)

	// set a tripwire
	require.Equal(sampling.LookbackParameter, 3, "these tests assume LookbackParameter=3")

	t.Run("happy path", func(t *testing.T) {
		ctx := NewVMContext(NewContextParams{
			Ancestors: tipSetsDescBlockHeight,
		})

		r, err := ctx.SampleChainRandomness(types.NewBlockHeight(uint64(20)))
		assert.NoError(err)
		assert.Equal([]byte(strconv.Itoa(17)), r)

		r, err = ctx.SampleChainRandomness(types.NewBlockHeight(uint64(3)))
		assert.NoError(err)
		assert.Equal([]byte(strconv.Itoa(0)), r)

		r, err = ctx.SampleChainRandomness(types.NewBlockHeight(uint64(10)))
		assert.NoError(err)
		assert.Equal([]byte(strconv.Itoa(7)), r)
	})

	t.Run("faults with height out of range", func(t *testing.T) {
		// edit `tipSetsDescBlockHeight` to include null blocks at heights 21
		// through 24
		baseBlock := tipSetsDescBlockHeight[1].ToSlice()[0]
		afterNull := types.NewBlockForTest(baseBlock, uint64(0))
		afterNull.Height += types.Uint64(uint64(5))
		afterNull.Ticket = []byte(strconv.Itoa(int(afterNull.Height)))
		modAncestors := append([]types.TipSet{types.RequireNewTipSet(require, afterNull)}, tipSetsDescBlockHeight...)

		// ancestor block heights:
		//
		// 25 20 19 18 17 16 15 14 13 12 11 10 9 8 7 6 5 4 3 2 1 0
		ctx := NewVMContext(NewContextParams{
			Ancestors: modAncestors,
		})

		// no tip set with height 30 exists in ancestors
		_, err := ctx.SampleChainRandomness(types.NewBlockHeight(uint64(30)))
		assert.Error(err)
	})

	t.Run("faults with lookback out of range", func(t *testing.T) {
		// ancestor block heights:
		//
		// 25 20
		ctx := NewVMContext(NewContextParams{
			Ancestors: tipSetsDescBlockHeight[:5],
		})

		// going back in time by `LookbackParameter`-number of tip sets from
		// block height 25 does not find us the genesis block
		_, err := ctx.SampleChainRandomness(types.NewBlockHeight(uint64(25)))
		assert.Error(err)
	})

	t.Run("falls back to genesis block", func(t *testing.T) {
		vmCtxParams := NewContextParams{
			Ancestors: tipSetsDescBlockHeight,
		}
		ctx := NewVMContext(vmCtxParams)
		r, err := ctx.SampleChainRandomness(types.NewBlockHeight(uint64(1))) // lookback height lower than all tipSetsDescBlockHeight
		assert.NoError(err)
		assert.Equal([]byte(strconv.Itoa(0)), r)
	})
}
