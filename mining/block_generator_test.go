package mining

import (
	"context"
	"errors"
	"testing"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gx/ipfs/QmcYBp5EDnJKfVN63F71rDTksvEf1cfijwCTWtw6bPG58T/go-hamt-ipld"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

func TestGenerate(t *testing.T) {
	// TODO fritz use core.FakeActor for state/contract tests for generate:
	//  - test nonces out of order
	//  - test nonce gap
}

func sharedSetupInitial() (*hamt.CborIpldStore, *core.MessagePool, *cid.Cid) {
	cst := hamt.NewCborStore()
	pool := core.NewMessagePool()
	// Install the fake actor so we can execute it.
	fakeActorCodeCid := types.AccountActorCodeCid
	return cst, pool, fakeActorCodeCid
}

func sharedSetup(t *testing.T) (state.Tree, *core.MessagePool, []types.Address) {
	require := require.New(t)
	newAddress := types.NewAddressForTestGetter()
	cst, pool, fakeActorCodeCid := sharedSetupInitial()

	// TODO: We don't need fake actors here, so these could be made real.
	//       And the NetworkAddress actor can/should be the real one.
	// Stick two fake actors in the state tree so they can talk.
	addr1, addr2, addr3 := newAddress(), newAddress(), newAddress()
	act1, act2, fakeNetAct := core.RequireNewFakeActor(require, fakeActorCodeCid), core.RequireNewFakeActor(require,
		fakeActorCodeCid), core.RequireNewFakeActor(require, fakeActorCodeCid)
	_, st := core.RequireMakeStateTree(require, cst, map[types.Address]*types.Actor{
		// Ensure core.NetworkAddress exists to prevent mining reward message failures.
		address.NetworkAddress: fakeNetAct,
		addr1: act1,
		addr2: act2,
	})
	return st, pool, []types.Address{addr1, addr2, addr3}
}

func TestApplyMessagesForSuccessTempAndPermFailures(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	newAddress := types.NewAddressForTestGetter()

	cst, _, fakeActorCodeCid := sharedSetupInitial()

	// Stick two fake actors in the state tree so they can talk.
	addr1, addr2 := newAddress(), newAddress()
	act1 := core.RequireNewFakeActor(require, fakeActorCodeCid)
	_, st := core.RequireMakeStateTree(require, cst, map[types.Address]*types.Actor{
		addr1: act1,
	})

	ctx := context.Background()

	// NOTE: it is important that each category (success, temporary failure, permanent failure) is represented below.
	// If a given message's category changes in the future, it needs to be replaced here in tests by another so we fully
	// exercise the categorization.
	// addr2 doesn't correspond to an extant account, so this will trigger errAccountNotFound -- a temporary failure.
	msg1 := types.NewMessage(addr2, addr1, 0, nil, "", nil)
	// This is actually okay and should result in a receipt
	msg2 := types.NewMessage(addr1, addr2, 0, nil, "", nil)
	// The following two are sending to self -- errSelfSend, a permanent error.
	msg3 := types.NewMessage(addr1, addr1, 1, nil, "", nil)
	msg4 := types.NewMessage(addr2, addr2, 1, nil, "", nil)

	messages := []*types.Message{msg1, msg2, msg3, msg4}

	res, err := core.ApplyMessages(ctx, messages, st, types.NewBlockHeight(0))

	assert.Len(res.PermanentFailures, 2)
	assert.Contains(res.PermanentFailures, msg3)
	assert.Contains(res.PermanentFailures, msg4)

	assert.Len(res.TemporaryFailures, 1)
	assert.Contains(res.TemporaryFailures, msg1)

	assert.Len(res.Results, 1)
	assert.Contains(res.SuccessfulMessages, msg2)

	assert.NoError(err)
}

func TestGenerateMultiBlockTipSet(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ctx := context.Background()
	newCid := types.NewCidForTestGetter()
	st, pool, addrs := sharedSetup(t)

	getStateTree := func(c context.Context, ts core.TipSet) (state.Tree, error) {
		return st, nil
	}
	getWeight := func(c context.Context, ts core.TipSet) (uint64, error) {
		pW, err := ts.ParentWeight()
		if err != nil {
			return uint64(0), err
		}
		return pW + uint64(len(ts))*core.ECV, nil
	}
	generator := NewBlockGenerator(pool, getStateTree, getWeight, core.ApplyMessages)

	parents := types.NewSortedCidSet(newCid())
	stateRoot := newCid()
	baseBlock1 := types.Block{
		Parents:      parents,
		Height:       types.Uint64(100),
		ParentWeight: types.Uint64(1000),
		StateRoot:    stateRoot,
	}
	baseBlock2 := types.Block{
		Parents:      parents,
		Height:       types.Uint64(100),
		ParentWeight: types.Uint64(1000),
		StateRoot:    stateRoot,
		Nonce:        1,
	}
	blk, err := generator.Generate(ctx, core.RequireNewTipSet(require, &baseBlock1, &baseBlock2), nil, 0, addrs[0])
	assert.NoError(err)

	assert.Len(blk.Messages, 1) // This is the mining reward.
	assert.Equal(types.Uint64(101), blk.Height)
	assert.Equal(types.Uint64(1020), blk.ParentWeight)
}

// After calling Generate, do the new block and new state of the message pool conform to our expectations?
func TestGeneratePoolBlockResults(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ctx := context.Background()
	newCid := types.NewCidForTestGetter()
	st, pool, addrs := sharedSetup(t)

	getStateTree := func(c context.Context, ts core.TipSet) (state.Tree, error) {
		return st, nil
	}
	getWeight := func(c context.Context, ts core.TipSet) (uint64, error) {
		pW, err := ts.ParentWeight()
		if err != nil {
			return uint64(0), err
		}
		return pW + uint64(len(ts))*core.ECV, nil
	}
	generator := NewBlockGenerator(pool, getStateTree, getWeight, core.ApplyMessages)

	// addr3 doesn't correspond to an extant account, so this will trigger errAccountNotFound -- a temporary failure.
	msg1 := types.NewMessage(addrs[2], addrs[0], 0, nil, "", nil)
	// This is actually okay and should result in a receipt
	msg2 := types.NewMessage(addrs[0], addrs[1], 0, nil, "", nil)
	// The following two are sending to self -- errSelfSend, a permanent error.
	msg3 := types.NewMessage(addrs[0], addrs[0], 1, nil, "", nil)
	msg4 := types.NewMessage(addrs[1], addrs[1], 0, nil, "", nil)
	pool.Add(msg1)
	pool.Add(msg2)
	pool.Add(msg3)
	pool.Add(msg4)

	assert.Len(pool.Pending(), 4)
	baseBlock := types.Block{
		Parents:   types.NewSortedCidSet(newCid()),
		Height:    types.Uint64(100),
		StateRoot: newCid(),
	}
	blk, err := generator.Generate(ctx, core.RequireNewTipSet(require, &baseBlock), nil, 0, addrs[0])
	assert.NoError(err)

	assert.Len(pool.Pending(), 1) // This is the temporary failure.
	assert.Contains(pool.Pending(), msg1)

	assert.Len(blk.Messages, 2) // This is the good message + the mining reward.

	// Is the mining reward first? This will fail 50% of the time if we don't force the reward to come first.
	assert.Equal(address.NetworkAddress, blk.Messages[0].From)
}

func TestGenerateSetsBasicFields(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ctx := context.Background()
	newCid := types.NewCidForTestGetter()

	st, pool, addrs := sharedSetup(t)

	getStateTree := func(c context.Context, ts core.TipSet) (state.Tree, error) {
		return st, nil
	}
	getWeight := func(c context.Context, ts core.TipSet) (uint64, error) {
		pW, err := ts.ParentWeight()
		if err != nil {
			return uint64(0), err
		}
		return pW + uint64(len(ts))*core.ECV, nil
	}
	generator := NewBlockGenerator(pool, getStateTree, getWeight, core.ApplyMessages)

	h := types.Uint64(100)
	pw := types.Uint64(1000)
	baseBlock := types.Block{
		Height:       h,
		ParentWeight: pw,
		StateRoot:    newCid(),
	}
	baseTipSet := core.RequireNewTipSet(require, &baseBlock)
	blk, err := generator.Generate(ctx, baseTipSet, nil, 0, addrs[0])
	assert.NoError(err)

	assert.Equal(h+1, blk.Height)
	assert.Equal(addrs[0], blk.Miner)

	blk, err = generator.Generate(ctx, baseTipSet, nil, 1, addrs[0])
	assert.NoError(err)

	assert.Equal(h+2, blk.Height)
	assert.Equal(pw+10.0, blk.ParentWeight)
	assert.Equal(addrs[0], blk.Miner)
}

func TestGenerateWithoutMessages(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ctx := context.Background()
	newCid := types.NewCidForTestGetter()

	st, pool, addrs := sharedSetup(t)

	getStateTree := func(c context.Context, ts core.TipSet) (state.Tree, error) {
		return st, nil
	}
	getWeight := func(c context.Context, ts core.TipSet) (uint64, error) {
		pW, err := ts.ParentWeight()
		if err != nil {
			return uint64(0), err
		}
		return pW + uint64(len(ts))*core.ECV, nil
	}
	generator := NewBlockGenerator(pool, getStateTree, getWeight, core.ApplyMessages)

	assert.Len(pool.Pending(), 0)
	baseBlock := types.Block{
		Parents:   types.NewSortedCidSet(newCid()),
		Height:    types.Uint64(100),
		StateRoot: newCid(),
	}
	blk, err := generator.Generate(ctx, core.RequireNewTipSet(require, &baseBlock), nil, 0, addrs[0])
	assert.NoError(err)

	assert.Len(pool.Pending(), 0) // This is the temporary failure.
	assert.Len(blk.Messages, 1)   // This is the mining reward.
}

// If something goes wrong while generating a new block, even as late as when flushing it,
// no block should be returned, and the message pool should not be pruned.
func TestGenerateError(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx := context.Background()
	newCid := types.NewCidForTestGetter()

	st, pool, addrs := sharedSetup(t)

	explodingGetStateTree := func(c context.Context, ts core.TipSet) (state.Tree, error) {
		stt := WrapStateTreeForTest(st)
		stt.TestFlush = func(ctx context.Context) (*cid.Cid, error) {
			return nil, errors.New("boom no flush")
		}

		return stt, nil
	}
	getWeight := func(c context.Context, ts core.TipSet) (uint64, error) {
		pW, err := ts.ParentWeight()
		if err != nil {
			return uint64(0), err
		}
		return pW + uint64(len(ts))*core.ECV, nil
	}
	generator := NewBlockGenerator(pool, explodingGetStateTree, getWeight, core.ApplyMessages)

	// This is actually okay and should result in a receipt
	msg := types.NewMessage(addrs[0], addrs[1], 0, nil, "", nil)
	pool.Add(msg)

	assert.Len(pool.Pending(), 1)
	baseBlock := types.Block{
		Parents:   types.NewSortedCidSet(newCid()),
		Height:    types.Uint64(100),
		StateRoot: newCid(),
	}
	baseTipSet := core.RequireNewTipSet(require, &baseBlock)
	blk, err := generator.Generate(ctx, baseTipSet, nil, 0, addrs[0])
	assert.Error(err, "boom")
	assert.Nil(blk)

	assert.Len(pool.Pending(), 1) // No messages are removed from the pool.
}

type StateTreeForTest struct {
	state.Tree
	TestFlush func(ctx context.Context) (*cid.Cid, error)
}

func WrapStateTreeForTest(st state.Tree) *StateTreeForTest {
	stt := StateTreeForTest{
		st,
		st.Flush,
	}
	return &stt
}

func (st *StateTreeForTest) Flush(ctx context.Context) (*cid.Cid, error) {
	return st.TestFlush(ctx)
}
