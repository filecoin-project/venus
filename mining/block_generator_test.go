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

	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"gx/ipfs/QmdtiofXbibTe6Day9ii5zjBZpSRm8vhfoerrNuY3sAQ7e/go-hamt-ipld"
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
	receipts, perm, temp, success, err := ApplyMessages(ctx, messages, st, types.NewBlockHeight(0))

	assert.Len(perm, 2)
	assert.Contains(perm, msg3)
	assert.Contains(perm, msg4)

	assert.Len(temp, 1)
	assert.Contains(temp, msg1)

	assert.Len(receipts, 1)
	assert.Contains(success, msg2)
	assert.NoError(err)
}

// After calling Generate, do the new block and new state of the message pool conform to our expectations?
func TestGeneratePoolBlockResults(t *testing.T) {
	assert := assert.New(t)

	ctx := context.Background()
	newCid := types.NewCidForTestGetter()
	st, pool, addrs := sharedSetup(t)

	getStateTree := func(c context.Context, stateRootCid *cid.Cid) (state.Tree, error) {
		return st, nil
	}
	generator := NewBlockGenerator(pool, getStateTree, ApplyMessages)

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
		Height:    uint64(100),
		StateRoot: newCid(),
	}
	blk, err := generator.Generate(ctx, &baseBlock, addrs[0])
	assert.NoError(err)

	assert.Len(pool.Pending(), 1) // This is the temporary failure.
	assert.Contains(pool.Pending(), msg1)

	assert.Len(blk.Messages, 2) // This is the good message + the mining reward.

	// Is the mining reward first? This will fail 50% of the time if we don't force the reward to come first.
	assert.Equal(address.NetworkAddress, blk.Messages[0].From)
}

func TestGenerateSetsBasicFields(t *testing.T) {
	assert := assert.New(t)

	ctx := context.Background()
	newCid := types.NewCidForTestGetter()

	st, pool, addrs := sharedSetup(t)

	getStateTree := func(c context.Context, stateRootCid *cid.Cid) (state.Tree, error) {
		return st, nil
	}
	generator := NewBlockGenerator(pool, getStateTree, ApplyMessages)

	h := uint64(100)
	baseBlock := types.Block{
		Height:    h,
		StateRoot: newCid(),
	}
	blk, err := generator.Generate(ctx, &baseBlock, addrs[0])
	assert.NoError(err)

	assert.Equal(h+1, blk.Height)
	assert.Equal(addrs[0], blk.Miner)
}

func TestGenerateWithoutMessages(t *testing.T) {
	assert := assert.New(t)

	ctx := context.Background()
	newCid := types.NewCidForTestGetter()

	st, pool, addrs := sharedSetup(t)

	getStateTree := func(c context.Context, stateRootCid *cid.Cid) (state.Tree, error) {
		return st, nil
	}
	generator := NewBlockGenerator(pool, getStateTree, ApplyMessages)

	assert.Len(pool.Pending(), 0)
	baseBlock := types.Block{
		Parents:   types.NewSortedCidSet(newCid()),
		Height:    uint64(100),
		StateRoot: newCid(),
	}
	blk, err := generator.Generate(ctx, &baseBlock, addrs[0])
	assert.NoError(err)

	assert.Len(pool.Pending(), 0) // This is the temporary failure.
	assert.Len(blk.Messages, 1)   // This is the mining reward.
}

// If something goes wrong while generating a new block, even as late as when flushing it,
// no block should be returned, and the message pool should not be pruned.
func TestGenerateError(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	newCid := types.NewCidForTestGetter()

	st, pool, addrs := sharedSetup(t)

	explodingGetStateTree := func(c context.Context, stateRootCid *cid.Cid) (state.Tree, error) {
		stt := WrapStateTreeForTest(st)
		stt.TestFlush = func(ctx context.Context) (*cid.Cid, error) {
			return nil, errors.New("boom no flush")
		}

		return stt, nil
	}
	generator := NewBlockGenerator(pool, explodingGetStateTree, ApplyMessages)

	// This is actually okay and should result in a receipt
	msg := types.NewMessage(addrs[0], addrs[1], 0, nil, "", nil)
	pool.Add(msg)

	assert.Len(pool.Pending(), 1)
	baseBlock := types.Block{
		Parents:   types.NewSortedCidSet(newCid()),
		Height:    uint64(100),
		StateRoot: newCid(),
	}
	blk, err := generator.Generate(ctx, &baseBlock, addrs[0])
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
