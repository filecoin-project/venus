package mining

import (
	"context"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gx/ipfs/QmQZadYTDF4ud9DdK85PH2vReJRzUM9YfVW4ReB1q2m51p/go-hamt-ipld"
	"gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore"
	sha256 "gx/ipfs/QmXTpwq2AkzQsPjKqFQDNY2bMdsAT53hUBETeyj8QRHTZU/sha256-simd"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	"gx/ipfs/QmcmpX42gtDv1fz24kau4wjS9hfwWj5VexWBKgGnWzsyag/go-ipfs-blockstore"
)

func Test_Mine(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	newCid := types.NewCidForTestGetter()
	stateRoot := newCid()
	baseBlock := &types.Block{Height: 2, StateRoot: stateRoot}
	tipSet := core.RequireNewTipSet(require, baseBlock)
	ctx, cancel := context.WithCancel(context.Background())

	st, pool, addrs, cst, bs := sharedSetup(t)
	getStateTree := func(c context.Context, ts core.TipSet) (state.Tree, error) {
		return st, nil
	}

	// Success case. TODO: this case isn't testing much.  Testing w.Mine
	// further needs a lot more attention.
	worker := NewDefaultWorker(pool, getStateTree, getWeightTest, core.ApplyMessages, &core.TestView{}, bs, cst, addrs[3], BlockTimeTest)
	outCh := make(chan Output)
	doSomeWorkCalled := false
	worker.createPoST = func() { doSomeWorkCalled = true }
	input := NewInput(tipSet)
	go worker.Mine(ctx, input, outCh)
	r := <-outCh
	assert.NoError(r.Err)
	assert.True(doSomeWorkCalled)
	cancel()

	// Block generation fails.
	ctx, cancel = context.WithCancel(context.Background())
	worker = NewDefaultWorker(pool, makeExplodingGetStateTree(st), getWeightTest, core.ApplyMessages, &core.TestView{}, bs, cst, addrs[3], BlockTimeTest)
	outCh = make(chan Output)
	doSomeWorkCalled = false
	worker.createPoST = func() { doSomeWorkCalled = true }
	input = NewInput(tipSet)
	go worker.Mine(ctx, input, outCh)
	r = <-outCh
	assert.Error(r.Err)
	assert.True(doSomeWorkCalled)
	cancel()

	// Sent empty tipset
	ctx, cancel = context.WithCancel(context.Background())
	worker = NewDefaultWorker(pool, getStateTree, getWeightTest, core.ApplyMessages, &core.TestView{}, bs, cst, addrs[3], BlockTimeTest)
	outCh = make(chan Output)
	doSomeWorkCalled = false
	worker.createPoST = func() { doSomeWorkCalled = true }
	input = NewInput(core.TipSet{})
	go worker.Mine(ctx, input, outCh)
	r = <-outCh
	assert.Error(r.Err)
	assert.False(doSomeWorkCalled)
	cancel()
}

func TestIsWinningTicket(t *testing.T) {
	assert := assert.New(t)

	cases := []struct {
		ticket     byte
		myPower    int64
		totalPower int64
		wins       bool
	}{
		{0x00, 1, 5, true},
		{0x30, 1, 5, true},
		{0x40, 1, 5, false},
		{0xF0, 1, 5, false},
		{0x00, 5, 5, true},
		{0x33, 5, 5, true},
		{0x44, 5, 5, true},
		{0xFF, 5, 5, true},
		{0x00, 0, 5, false},
		{0x33, 0, 5, false},
		{0x44, 0, 5, false},
		{0xFF, 0, 5, false},
	}

	for _, c := range cases {
		ticket := [sha256.Size]byte{}
		ticket[0] = c.ticket
		r := isWinningTicket(ticket[:], c.myPower, c.totalPower)
		assert.Equal(c.wins, r, "%+v", c)
	}
}

// worker test
func TestCreateChallenge(t *testing.T) {
	assert := assert.New(t)

	cases := []struct {
		parentTickets  [][]byte
		nullBlockCount uint64
		challenge      string
	}{
		// From https://www.di-mgt.com.au/sha_testvectors.html
		{[][]byte{[]byte("ac"), []byte("ab"), []byte("xx")}, uint64('c'),
			"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
		{[][]byte{[]byte("z"), []byte("x"), []byte("abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnop")},
			uint64('q'), "248d6a61d20638b8e5c026930c3e6039a33ce45964ff2167f6ecedd419db06c1"},
		{[][]byte{[]byte("abcdefghbcdefghicdefghijdefghijkefghijklfghijklmghijklmnhijklmnoijklmnopjklmnopqklmnopqrlmnopqrsmnopqrstnopqrst"), []byte("z"), []byte("x")},
			uint64('u'), "cf5b16a778af8380036ce59e7b0492370b249b11e8f07a51afac45037afee9d1"},
	}

	for _, c := range cases {
		decoded, err := hex.DecodeString(c.challenge)
		assert.NoError(err)

		parents := core.TipSet{}
		for _, t := range c.parentTickets {
			b := types.Block{Ticket: t}
			parents.AddBlock(&b)
		}
		r, err := createChallenge(parents, c.nullBlockCount)
		assert.NoError(err)
		assert.Equal(decoded, r)
	}
}

var seed = types.GenerateKeyInfoSeed()
var ki = types.MustGenerateKeyInfo(10, seed)
var mockSigner = types.NewMockSigner(ki)

func TestGenerate(t *testing.T) {
	// TODO use core.FakeActor for state/contract tests for generate:
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

func sharedSetup(t *testing.T) (state.Tree, *core.MessagePool, []address.Address, *hamt.CborIpldStore, blockstore.Blockstore) {
	require := require.New(t)
	cst, pool, fakeActorCodeCid := sharedSetupInitial()
	vms := core.VMStorage()
	d := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(d)

	// TODO: We don't need fake actors here, so these could be made real.
	//       And the NetworkAddress actor can/should be the real one.
	// Stick two fake actors in the state tree so they can talk.
	addr1, addr2, addr3, addr4, addr5 := mockSigner.Addresses[0], mockSigner.Addresses[1], mockSigner.Addresses[2], mockSigner.Addresses[3], mockSigner.Addresses[4]
	act1 := core.RequireNewFakeActor(require, vms, addr1, fakeActorCodeCid)
	act2 := core.RequireNewFakeActor(require, vms, addr2, fakeActorCodeCid)
	fakeNetAct := core.RequireNewFakeActor(require, vms, addr3, fakeActorCodeCid)
	minerAct := core.RequireNewMinerActor(require, vms, addr4, addr5, []byte{}, 10, core.RequireRandomPeerID(), types.NewAttoFILFromFIL(10000))
	minerOwner := core.RequireNewFakeActor(require, vms, addr5, fakeActorCodeCid)
	_, st := core.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
		// Ensure core.NetworkAddress exists to prevent mining reward message failures.
		address.NetworkAddress: fakeNetAct,

		addr1: act1,
		addr2: act2,
		addr4: minerAct,
		addr5: minerOwner,
	})
	return st, pool, []address.Address{addr1, addr2, addr3, addr4, addr5}, cst, bs
}

// TODO this test belongs in core, it calls ApplyMessages
func TestApplyMessagesForSuccessTempAndPermFailures(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	vms := core.VMStorage()

	cst, _, fakeActorCodeCid := sharedSetupInitial()

	// Stick two fake actors in the state tree so they can talk.
	addr1, addr2 := mockSigner.Addresses[0], mockSigner.Addresses[1]
	act1 := core.RequireNewFakeActor(require, vms, addr1, fakeActorCodeCid)
	_, st := core.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
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
	st, pool, addrs, cst, bs := sharedSetup(t)
	getStateTree := func(c context.Context, ts core.TipSet) (state.Tree, error) {
		return st, nil
	}
	worker := NewDefaultWorker(pool, getStateTree, getWeightTest, core.ApplyMessages, &core.TestView{}, bs, cst, addrs[3], BlockTimeTest)

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
	blk, err := worker.Generate(ctx, core.RequireNewTipSet(require, &baseBlock1, &baseBlock2), nil, 0)
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
	st, pool, addrs, cst, bs := sharedSetup(t)

	getStateTree := func(c context.Context, ts core.TipSet) (state.Tree, error) {
		return st, nil
	}
	worker := NewDefaultWorker(pool, getStateTree, getWeightTest, core.ApplyMessages, &core.TestView{}, bs, cst, addrs[3], BlockTimeTest)

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
	blk, err := worker.Generate(ctx, core.RequireNewTipSet(require, &baseBlock), nil, 0)
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

	st, pool, addrs, cst, bs := sharedSetup(t)

	getStateTree := func(c context.Context, ts core.TipSet) (state.Tree, error) {
		return st, nil
	}
	worker := NewDefaultWorker(pool, getStateTree, getWeightTest, core.ApplyMessages, &core.TestView{}, bs, cst, addrs[3], BlockTimeTest)

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
	blk, err := worker.Generate(ctx, baseTipSet, nil, 0)
	assert.NoError(err)

	assert.Equal(h+1, blk.Height)
	assert.Equal(addrs[3], blk.Miner)

	blk, err = worker.Generate(ctx, baseTipSet, nil, 1)
	assert.NoError(err)

	assert.Equal(h+2, blk.Height)
	assert.Equal(wNum+10.0, blk.ParentWeightNum)
	assert.Equal(wDenom, blk.ParentWeightDenom)
	assert.Equal(addrs[3], blk.Miner)
}

func TestGenerateWithoutMessages(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ctx := context.Background()
	newCid := types.NewCidForTestGetter()

	st, pool, addrs, cst, bs := sharedSetup(t)
	getStateTree := func(c context.Context, ts core.TipSet) (state.Tree, error) {
		return st, nil
	}
	worker := NewDefaultWorker(pool, getStateTree, getWeightTest, core.ApplyMessages, &core.TestView{}, bs, cst, addrs[3], BlockTimeTest)

	assert.Len(pool.Pending(), 0)
	baseBlock := types.Block{
		Parents:   types.NewSortedCidSet(newCid()),
		Height:    types.Uint64(100),
		StateRoot: newCid(),
	}
	blk, err := worker.Generate(ctx, core.RequireNewTipSet(require, &baseBlock), nil, 0)
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

	st, pool, addrs, cst, bs := sharedSetup(t)

	worker := NewDefaultWorker(pool, makeExplodingGetStateTree(st), getWeightTest, core.ApplyMessages, &core.TestView{}, bs, cst, addrs[3], BlockTimeTest)

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
	blk, err := worker.Generate(ctx, baseTipSet, nil, 0)
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

func getWeightTest(c context.Context, ts core.TipSet) (uint64, uint64, error) {
	num, den, err := ts.ParentWeight()
	if err != nil {
		return uint64(0), uint64(0), err
	}
	return num + uint64(int64(len(ts))*int64(core.ECV)), den, nil
}

func makeExplodingGetStateTree(st state.Tree) func(context.Context, core.TipSet) (state.Tree, error) {
	return func(c context.Context, ts core.TipSet) (state.Tree, error) {
		stt := WrapStateTreeForTest(st)
		stt.TestFlush = func(ctx context.Context) (*cid.Cid, error) {
			return nil, errors.New("boom no flush")
		}

		return stt, nil
	}
}
