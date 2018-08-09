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

	"gx/ipfs/QmXJkSRxXHeAGmQJENct16anrKZHNECbmUoC7hMuCjLni6/go-hamt-ipld"
	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	"gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore"

	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/vm"
)

var seed = types.GenerateKeyInfoSeed()
var ki = types.MustGenerateKeyInfo(10, seed)
var mockSigner = types.NewMockSigner(ki)

func TestGenerate(t *testing.T) {
	// TODO fritz use core.FakeActor for state/contract tests for generate:
	//  - test nonces out of order
	//  - test nonce gap
}

func sharedSetupInitial() (*hamt.CborIpldStore, *core.MessagePool, datastore.Datastore, *cid.Cid) {
	cst := hamt.NewCborStore()
	pool := core.NewMessagePool()
	// Install the fake actor so we can execute it.
	fakeActorCodeCid := types.AccountActorCodeCid
	r := repo.NewInMemoryRepo()
	ds := r.Datastore()
	return cst, pool, ds, fakeActorCodeCid
}

func sharedSetup(t *testing.T) (state.Tree, datastore.Datastore, *core.MessagePool, []types.Address) {
	require := require.New(t)
	cst, pool, ds, fakeActorCodeCid := sharedSetupInitial()
	vms := vm.NewStorageMap(ds)

	// TODO: We don't need fake actors here, so these could be made real.
	//       And the NetworkAddress actor can/should be the real one.
	// Stick two fake actors in the state tree so they can talk.
	addr1, addr2, addr3, addr4 := mockSigner.Addresses[0], mockSigner.Addresses[1], mockSigner.Addresses[2], mockSigner.Addresses[3]
	act1, act2, fakeNetAct := core.RequireNewFakeActor(require, vms, addr1, fakeActorCodeCid), core.RequireNewFakeActor(require,
		vms, addr2, fakeActorCodeCid), core.RequireNewFakeActor(require, vms, addr3, fakeActorCodeCid)
	minerAct := core.RequireNewMinerActor(require, vms, addr4, addr1, []byte{}, types.NewBytesAmount(10000), core.RequireRandomPeerID(), types.NewAttoFILFromFIL(10000))
	_, st := core.RequireMakeStateTree(require, cst, map[types.Address]*types.Actor{
		// Ensure core.NetworkAddress exists to prevent mining reward message failures.
		address.NetworkAddress: fakeNetAct,
		addr1: act1,
		addr2: act2,
		addr4: minerAct,
	})
	return st, ds, pool, []types.Address{addr1, addr2, addr3, addr4}
}

func TestApplyMessagesForSuccessTempAndPermFailures(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	cst, _, ds, fakeActorCodeCid := sharedSetupInitial()
	vms := vm.NewStorageMap(ds)

	// Stick two fake actors in the state tree so they can talk.
	addr1, addr2 := mockSigner.Addresses[0], mockSigner.Addresses[1]
	act1 := core.RequireNewFakeActor(require, vms, addr1, fakeActorCodeCid)
	_, st := core.RequireMakeStateTree(require, cst, map[types.Address]*types.Actor{
		addr1: act1,
	})

	ctx := context.Background()

	// NOTE: it is important that each category (success, temporary failure, permanent failure) is represented below.
	// If a given message's category changes in the future, it needs to be replaced here in tests by another so we fully
	// exercise the categorization.
	// addr2 doesn't correspond to an extant account, so this will trigger errAccountNotFound -- a temporary failure.
	msg1 := types.NewMessage(addr2, addr1, 0, nil, "", nil)
	smsg1, err := types.NewSignedMessage(*msg1, &mockSigner)
	require.NoError(err)

	// This is actually okay and should result in a receipt
	msg2 := types.NewMessage(addr1, addr2, 0, nil, "", nil)
	smsg2, err := types.NewSignedMessage(*msg2, &mockSigner)
	require.NoError(err)

	// The following two are sending to self -- errSelfSend, a permanent error.
	msg3 := types.NewMessage(addr1, addr1, 1, nil, "", nil)
	smsg3, err := types.NewSignedMessage(*msg3, &mockSigner)
	require.NoError(err)

	msg4 := types.NewMessage(addr2, addr2, 1, nil, "", nil)
	smsg4, err := types.NewSignedMessage(*msg4, &mockSigner)
	require.NoError(err)

	messages := []*types.SignedMessage{smsg1, smsg2, smsg3, smsg4}

	res, err := core.ApplyMessages(ctx, messages, st, vms, types.NewBlockHeight(0))

	assert.Len(res.PermanentFailures, 2)
	assert.Contains(res.PermanentFailures, smsg3)
	assert.Contains(res.PermanentFailures, smsg4)

	assert.Len(res.TemporaryFailures, 1)
	assert.Contains(res.TemporaryFailures, smsg1)

	assert.Len(res.Results, 1)
	assert.Contains(res.SuccessfulMessages, smsg2)

	assert.NoError(err)
}

func TestGenerateMultiBlockTipSet(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ctx := context.Background()
	newCid := types.NewCidForTestGetter()
	st, ds, pool, addrs := sharedSetup(t)

	getStateTree := func(c context.Context, ts core.TipSet) (state.Tree, datastore.Datastore, error) {
		return st, ds, nil
	}
	getWeight := func(c context.Context, ts core.TipSet) (uint64, uint64, error) {
		num, den, err := ts.ParentWeight()
		if err != nil {
			return uint64(0), uint64(0), err
		}
		return num + uint64(int64(len(ts))*int64(core.ECV)), den, nil
	}
	generator := NewBlockGenerator(pool, getStateTree, getWeight, core.ApplyMessages, &core.TestView{})

	parents := types.NewSortedCidSet(newCid())
	stateRoot := newCid()
	baseBlock1 := types.Block{
		Parents:         parents,
		Height:          types.Uint64(100),
		ParentWeightNum: types.Uint64(1000),
		StateRoot:       stateRoot,
	}
	baseBlock2 := types.Block{
		Parents:         parents,
		Height:          types.Uint64(100),
		ParentWeightNum: types.Uint64(1000),
		StateRoot:       stateRoot,
		Nonce:           1,
	}
	blk, err := generator.Generate(ctx, core.RequireNewTipSet(require, &baseBlock1, &baseBlock2), nil, 0, addrs[0], addrs[3])
	assert.NoError(err)

	assert.Len(blk.Messages, 1) // This is the mining reward.
	assert.Equal(types.Uint64(101), blk.Height)
	assert.Equal(types.Uint64(1020), blk.ParentWeightNum)
}

// After calling Generate, do the new block and new state of the message pool conform to our expectations?
func TestGeneratePoolBlockResults(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ctx := context.Background()
	newCid := types.NewCidForTestGetter()
	st, ds, pool, addrs := sharedSetup(t)

	getStateTree := func(c context.Context, ts core.TipSet) (state.Tree, datastore.Datastore, error) {
		return st, ds, nil
	}
	getWeight := func(c context.Context, ts core.TipSet) (uint64, uint64, error) {
		num, den, err := ts.ParentWeight()
		if err != nil {
			return uint64(0), uint64(0), err
		}
		return num + uint64(int64(len(ts))*int64(core.ECV)), den, nil
	}
	generator := NewBlockGenerator(pool, getStateTree, getWeight, core.ApplyMessages, &core.TestView{})

	// addr3 doesn't correspond to an extant account, so this will trigger errAccountNotFound -- a temporary failure.
	msg1 := types.NewMessage(addrs[2], addrs[0], 0, nil, "", nil)
	smsg1, err := types.NewSignedMessage(*msg1, &mockSigner)
	require.NoError(err)

	// This is actually okay and should result in a receipt
	msg2 := types.NewMessage(addrs[0], addrs[1], 0, nil, "", nil)
	smsg2, err := types.NewSignedMessage(*msg2, &mockSigner)
	require.NoError(err)

	// The following two are sending to self -- errSelfSend, a permanent error.
	msg3 := types.NewMessage(addrs[0], addrs[0], 1, nil, "", nil)
	smsg3, err := types.NewSignedMessage(*msg3, &mockSigner)
	require.NoError(err)

	msg4 := types.NewMessage(addrs[1], addrs[1], 0, nil, "", nil)
	smsg4, err := types.NewSignedMessage(*msg4, &mockSigner)
	require.NoError(err)

	pool.Add(smsg1)
	pool.Add(smsg2)
	pool.Add(smsg3)
	pool.Add(smsg4)

	assert.Len(pool.Pending(), 4)
	baseBlock := types.Block{
		Parents:   types.NewSortedCidSet(newCid()),
		Height:    types.Uint64(100),
		StateRoot: newCid(),
	}
	blk, err := generator.Generate(ctx, core.RequireNewTipSet(require, &baseBlock), nil, 0, addrs[0], addrs[3])
	assert.NoError(err)

	assert.Len(pool.Pending(), 1) // This is the temporary failure.
	assert.Contains(pool.Pending(), smsg1)

	assert.Len(blk.Messages, 2) // This is the good message + the mining reward.

	// Is the mining reward first? This will fail 50% of the time if we don't force the reward to come first.
	assert.Equal(address.NetworkAddress, blk.Messages[0].From)
}

func TestGenerateSetsBasicFields(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ctx := context.Background()
	newCid := types.NewCidForTestGetter()

	st, ds, pool, addrs := sharedSetup(t)

	getStateTree := func(c context.Context, ts core.TipSet) (state.Tree, datastore.Datastore, error) {
		return st, ds, nil
	}
	getWeight := func(c context.Context, ts core.TipSet) (uint64, uint64, error) {
		num, den, err := ts.ParentWeight()
		if err != nil {
			return uint64(0), uint64(0), err
		}
		return num + uint64(int64(len(ts))*int64(core.ECV)), den, nil
	}
	generator := NewBlockGenerator(pool, getStateTree, getWeight, core.ApplyMessages, &core.TestView{})

	h := types.Uint64(100)
	wNum := types.Uint64(1000)
	wDenom := types.Uint64(1)
	baseBlock := types.Block{
		Height:            h,
		ParentWeightNum:   wNum,
		ParentWeightDenom: wDenom,
		StateRoot:         newCid(),
	}
	baseTipSet := core.RequireNewTipSet(require, &baseBlock)
	blk, err := generator.Generate(ctx, baseTipSet, nil, 0, addrs[0], addrs[3])
	assert.NoError(err)

	assert.Equal(h+1, blk.Height)
	assert.Equal(addrs[0], blk.Reward)
	assert.Equal(addrs[3], blk.Miner)

	blk, err = generator.Generate(ctx, baseTipSet, nil, 1, addrs[0], addrs[3])
	assert.NoError(err)

	assert.Equal(h+2, blk.Height)
	assert.Equal(wNum+10.0, blk.ParentWeightNum)
	assert.Equal(wDenom, blk.ParentWeightDenom)
	assert.Equal(addrs[0], blk.Reward)
	assert.Equal(addrs[3], blk.Miner)
}

func TestGenerateWithoutMessages(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ctx := context.Background()
	newCid := types.NewCidForTestGetter()

	st, ds, pool, addrs := sharedSetup(t)

	getStateTree := func(c context.Context, ts core.TipSet) (state.Tree, datastore.Datastore, error) {
		return st, ds, nil
	}
	getWeight := func(c context.Context, ts core.TipSet) (uint64, uint64, error) {
		num, den, err := ts.ParentWeight()
		if err != nil {
			return uint64(0), uint64(0), err
		}
		return num + uint64(int64(len(ts))*int64(core.ECV)), den, nil
	}
	generator := NewBlockGenerator(pool, getStateTree, getWeight, core.ApplyMessages, &core.TestView{})

	assert.Len(pool.Pending(), 0)
	baseBlock := types.Block{
		Parents:   types.NewSortedCidSet(newCid()),
		Height:    types.Uint64(100),
		StateRoot: newCid(),
	}
	blk, err := generator.Generate(ctx, core.RequireNewTipSet(require, &baseBlock), nil, 0, addrs[0], addrs[3])
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

	st, ds, pool, addrs := sharedSetup(t)

	explodingGetStateTree := func(c context.Context, ts core.TipSet) (state.Tree, datastore.Datastore, error) {
		stt := WrapStateTreeForTest(st)
		stt.TestFlush = func(ctx context.Context) (*cid.Cid, error) {
			return nil, errors.New("boom no flush")
		}

		return stt, ds, nil
	}
	getWeight := func(c context.Context, ts core.TipSet) (uint64, uint64, error) {
		num, den, err := ts.ParentWeight()
		if err != nil {
			return uint64(0), uint64(0), err
		}
		return num + uint64(int64(len(ts))*int64(core.ECV)), den, nil
	}

	generator := NewBlockGenerator(pool, explodingGetStateTree, getWeight, core.ApplyMessages, &core.TestView{})

	// This is actually okay and should result in a receipt
	msg := types.NewMessage(addrs[0], addrs[1], 0, nil, "", nil)
	smsg, err := types.NewSignedMessage(*msg, &mockSigner)
	require.NoError(err)
	pool.Add(smsg)

	assert.Len(pool.Pending(), 1)
	baseBlock := types.Block{
		Parents:   types.NewSortedCidSet(newCid()),
		Height:    types.Uint64(100),
		StateRoot: newCid(),
	}
	baseTipSet := core.RequireNewTipSet(require, &baseBlock)
	blk, err := generator.Generate(ctx, baseTipSet, nil, 0, addrs[0], addrs[3])
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
