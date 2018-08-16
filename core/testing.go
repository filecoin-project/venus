package core

import (
	"context"
	"time"

	"gx/ipfs/QmU5VurujVopGNSxBbuBqC7gr12UarswyGhi9iwghRvi5P/go_rng"
	hamt "gx/ipfs/QmV1m7odB89Na2hw8YWK4TbP8NkotBt4jMTQaiqgYTdAm3/go-hamt-ipld"
	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	"gx/ipfs/QmcD7SqfyQyA91TZUQ7VPRYbGarxmY7EsQewVYMuN5LNSv/go-ipfs-blockstore"
	"gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"
	"gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer/test"
	"gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
	"github.com/stretchr/testify/require"

	"math/big"
	"testing"
)

// MkChild creates a new block with parent, blk, and supplied nonce.
func MkChild(blks []*types.Block, stateRoot *cid.Cid, nonce uint64) *types.Block {
	var weight uint64
	var height uint64
	var parents types.SortedCidSet
	weight = uint64(len(blks))*10 + uint64(blks[0].ParentWeightNum)
	height = uint64(blks[0].Height) + 1
	parents = types.SortedCidSet{}
	for _, blk := range blks {
		(&parents).Add(blk.Cid())
	}
	return &types.Block{
		Parents:           parents,
		Height:            types.Uint64(height),
		ParentWeightNum:   types.Uint64(weight),
		ParentWeightDenom: types.Uint64(1),
		Nonce:             types.Uint64(nonce),
		StateRoot:         stateRoot,
		Messages:          []*types.SignedMessage{},
		MessageReceipts:   []*types.MessageReceipt{},
	}
}

// AddChain creates and processes new, empty chain of length, beginning from blks.
func AddChain(ctx context.Context, processNewBlock NewBlockProcessor, loadStateTreeTS AggregateStateTreeComputer, blks []*types.Block, length int) (*types.Block, error) {
	ts, err := NewTipSet(blks...)
	if err != nil {
		return nil, err
	}
	st, _, err := loadStateTreeTS(ctx, ts)
	if err != nil {
		return nil, err
	}
	stateRoot, err := st.Flush(ctx)
	if err != nil {
		return nil, err
	}
	l := uint64(length)
	var blk *types.Block
	for i := uint64(0); i < l; i++ {
		blk = MkChild(blks, stateRoot, i)
		_, err := processNewBlock(ctx, blk)
		if err != nil {
			return nil, err
		}
		blks = []*types.Block{blk}
	}
	return blks[0], nil
}

// AddChainBinomBlocksPerEpoch creates and processes a new chain without messages, the
// given state generation function and block processors, and the input length.
// The chain is based at the tipset "ts".  The number of blocks mined
// in each epoch is drawn from the binomial distribution where n = num_miners and
// p = 1/n.  Concretely this distribution corresponds to the configuration where
// all miners have the same storage power.
func AddChainBinomBlocksPerEpoch(ctx context.Context, processNewBlock NewBlockProcessor, loadStateTreeTS AggregateStateTreeComputer, ts TipSet, numMiners, epochs int) (TipSet, error) {
	st, _, err := loadStateTreeTS(ctx, ts)
	if err != nil {
		return nil, err
	}
	stateRoot, err := st.Flush(ctx)
	if err != nil {
		return nil, err
	}

	// Initialize epoch traversal.
	l := uint64(epochs)
	var lastNull uint64
	var head TipSet
	blks := ts.ToSlice()
	brng := rng.NewBinomialGenerator(time.Now().UnixNano())
	n := int64(numMiners)
	p := float64(1) / float64(n)

	// Construct a tipset for each epoch.
	for i := uint64(0); i < l; i++ {
		head = TipSet{}
		// Draw number of blocks per TS from binom distribution.
		nBlks := brng.Binomial(n, p)
		if nBlks == int64(0) {
			lastNull += uint64(1)
		}

		// Construct each block and force the chain manager to process them.
		for j := int64(0); j < nBlks; j++ {
			blk := MkChild(blks, stateRoot, uint64(j))
			if lastNull > 0 { // TODO better include null block handling direcetly in MkChild interface
				blk.Height = blk.Height + types.Uint64(lastNull)
			}
			_, err := processNewBlock(ctx, blk)
			if err != nil {
				return nil, err
			}
			err = head.AddBlock(blk)
			if err != nil {
				return nil, err
			}
		}

		// Update chain head and null block count.
		if nBlks > int64(0) {
			lastNull = 0
			blks = head.ToSlice()
		}
	}
	return head, nil
}

// RequireMakeStateTree takes a map of addresses to actors and stores them on
// the state tree, requiring that all its steps succeed.
func RequireMakeStateTree(require *require.Assertions, cst *hamt.CborIpldStore, acts map[types.Address]*types.Actor) (*cid.Cid, state.Tree) {
	ctx := context.Background()
	t := state.NewEmptyStateTreeWithActors(cst, builtin.Actors)

	for addr, act := range acts {
		err := t.SetActor(ctx, addr, act)
		require.NoError(err)
	}

	c, err := t.Flush(ctx)
	require.NoError(err)

	return c, t
}

// RequireNewEmptyActor creates a new empty actor with the given starting
// value and requires that its steps succeed.
func RequireNewEmptyActor(require *require.Assertions, value *types.AttoFIL) *types.Actor {
	return &types.Actor{Balance: value}
}

// RequireNewAccountActor creates a new account actor with the given starting
// value and requires that its steps succeed.
func RequireNewAccountActor(require *require.Assertions, value *types.AttoFIL) *types.Actor {
	act, err := account.NewActor(value)
	require.NoError(err)
	return act
}

// RequireNewMinerActor creates a new miner actor with the given owner, pledge, and collateral,
// and requires that its steps succeed.
func RequireNewMinerActor(require *require.Assertions, vms vm.StorageMap, addr types.Address, owner types.Address, key []byte, pledge *types.BytesAmount, pid peer.ID, coll *types.AttoFIL) *types.Actor {
	act := types.NewActor(types.MinerActorCodeCid, types.NewZeroAttoFIL())
	storage := vms.NewStorage(addr, act)
	initializerData := miner.NewState(owner, key, pledge, pid, coll)
	err := (&miner.Actor{}).InitializeState(storage, initializerData)
	require.NoError(storage.Flush())
	require.NoError(err)
	return act
}

// RequireNewFakeActor instantiates and returns a new fake actor and requires
// that its steps succeed.
func RequireNewFakeActor(require *require.Assertions, vms vm.StorageMap, addr types.Address, codeCid *cid.Cid) *types.Actor {
	return RequireNewFakeActorWithTokens(require, vms, addr, codeCid, types.NewAttoFILFromFIL(100))
}

// RequireNewFakeActorWithTokens instantiates and returns a new fake actor and requires
// that its steps succeed.
func RequireNewFakeActorWithTokens(require *require.Assertions, vms vm.StorageMap, addr types.Address, codeCid *cid.Cid, amt *types.AttoFIL) *types.Actor {
	act := types.NewActor(codeCid, amt)
	store := vms.NewStorage(addr, act)
	err := (&actor.FakeActor{}).InitializeState(store, &actor.FakeActorStorage{})
	require.NoError(err)
	require.NoError(vms.Flush())
	return act
}

// RequireRandomPeerID returns a new libp2p peer ID or panics.
func RequireRandomPeerID() peer.ID {
	pid, err := testutil.RandPeerID()
	if err != nil {
		panic(err)
	}

	return pid
}

// RequireNewTipSet instantiates and returns a new tipset of the given blocks
// and requires that the setup validation succeed.
func RequireNewTipSet(require *require.Assertions, blks ...*types.Block) TipSet {
	ts, err := NewTipSet(blks...)
	require.NoError(err)
	return ts
}

// RequireTipSetAdd adds the input block to the tipset and requires that no
// errors occur.
func RequireTipSetAdd(require *require.Assertions, blk *types.Block, ts TipSet) {
	err := ts.AddBlock(blk)
	require.NoError(err)
}

// RequireBestBlock ensures that there is a single block in the heaviest tipset
// and returns it.
func RequireBestBlock(cm *ChainManager, t *testing.T) *types.Block {
	require := require.New(t)
	heaviest := cm.GetHeaviestTipSet()
	require.Equal(1, len(heaviest))
	return heaviest.ToSlice()[0]
}

// MustGetNonce returns the next nonce for an actor at the given address or panics.
func MustGetNonce(st state.Tree, a types.Address) uint64 {
	mp := NewMessagePool()
	nonce, err := NextNonce(context.Background(), st, mp, a)
	if err != nil {
		panic(err)
	}
	return nonce
}

// MustAdd adds the given messages to the messagepool or panics if it
// cannot.
func MustAdd(p *MessagePool, msgs ...*types.SignedMessage) {
	for _, m := range msgs {
		if _, err := p.Add(m); err != nil {
			panic(err)
		}
	}
}

// MustSign signs a given address with the provided mocksigner or panics if it
// cannot.
func MustSign(s types.MockSigner, msgs ...*types.Message) []*types.SignedMessage {
	var smsgs []*types.SignedMessage
	for _, m := range msgs {
		sm, err := types.NewSignedMessage(*m, &s)
		if err != nil {
			panic(err)
		}
		smsgs = append(smsgs, sm)
	}
	return smsgs
}

// MustConvertParams abi encodes the given parameters into a byte array (or panics)
func MustConvertParams(params ...interface{}) []byte {
	vals, err := abi.ToValues(params)
	if err != nil {
		panic(err)
	}

	out, err := abi.EncodeValues(vals)
	if err != nil {
		panic(err)
	}
	return out
}

// NewChainWithMessages creates a chain of tipsets containing the given messages
// and stores them in the given store.  Note the msg arguments are slices of
// slices of messages -- each slice of slices goes into a successive tipset,
// and each slice within this slice goes into a block of that tipset
func NewChainWithMessages(store *hamt.CborIpldStore, root TipSet, msgSets ...[][]*types.SignedMessage) []TipSet {
	tipSets := []TipSet{}
	parents := root

	// only add root to the chain if it is not the zero-valued-tipset
	if len(parents) != 0 {
		for _, blk := range parents {
			MustPut(store, blk)
		}
		tipSets = append(tipSets, parents)
	}

	for _, tsMsgs := range msgSets {
		height, _ := parents.Height()
		ts := TipSet{}
		// If a message set does not contain a slice of messages then
		// add a tipset with no messages and a single block to the chain
		if len(tsMsgs) == 0 {
			child := &types.Block{
				Height:  types.Uint64(height + 1),
				Parents: parents.ToSortedCidSet(),
			}
			MustPut(store, child)
			ts[child.Cid().String()] = child
		}
		for _, msgs := range tsMsgs {
			child := &types.Block{
				Messages: msgs,
				Parents:  parents.ToSortedCidSet(),
				Height:   types.Uint64(height + 1),
			}
			MustPut(store, child)
			ts[child.Cid().String()] = child
		}
		tipSets = append(tipSets, ts)
		parents = ts
	}

	return tipSets
}

// MustPut stores the thingy in the store or panics if it cannot.
func MustPut(store *hamt.CborIpldStore, thingy interface{}) *cid.Cid {
	cid, err := store.Put(context.Background(), thingy)
	if err != nil {
		panic(err)
	}
	return cid
}

// MustDecodeCid decodes a string to a Cid pointer, panicking on error
func MustDecodeCid(cidStr string) *cid.Cid {
	decode, err := cid.Decode(cidStr)
	if err != nil {
		panic(err)
	}

	return decode
}

// VMStorage creates a new storage object backed by an in memory datastore
func VMStorage() vm.StorageMap {
	return vm.NewStorageMap(blockstore.NewBlockstore(datastore.NewMapDatastore()))
}

// CreateStorages creates an empty state tree and storage map.
func CreateStorages(ctx context.Context, t *testing.T) (state.Tree, vm.StorageMap) {
	cst := hamt.NewCborStore()
	d := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(d)
	blk, err := InitGenesis(cst, bs)
	require.NoError(t, err)

	st, err := state.LoadStateTree(ctx, cst, blk.StateRoot, builtin.Actors)
	require.NoError(t, err)

	vms := vm.NewStorageMap(bs)

	return st, vms
}

// TestView is an implementation of stateView used for testing the chain
// manager.  It provides a consistent view that the storage market
// stores 1 byte and all miners store 0 bytes regardless of inputs.
type TestView struct{}

var _ PowerTableView = &TestView{}

// Total always returns 1.
func (tv *TestView) Total(ctx context.Context, st state.Tree, cstore *hamt.CborIpldStore) (uint64, error) {
	return uint64(1), nil
}

// Miner always returns 0.
func (tv *TestView) Miner(ctx context.Context, st state.Tree, cstore *hamt.CborIpldStore, mAddr types.Address) (uint64, error) {
	return uint64(0), nil
}

// HasPower always returns true.
func (tv *TestView) HasPower(ctx context.Context, st state.Tree, cstore *hamt.CborIpldStore, mAddr types.Address) bool {
	return true
}

// CreateMinerWithPower uses storage market functionality to mine the messages needed to create a miner, ask, bid, and deal, and then commit that deal to give the miner power.
// If the power is nil, this method will just create the miner.
// The returned block and nonce should be used in subsequent calls to this method.
func CreateMinerWithPower(ctx context.Context, t *testing.T, cm *ChainManager, lastBlock *types.Block, sn types.MockSigner, nonce uint64, rewardAddress types.Address, power *types.BytesAmount) (types.Address, *types.Block, uint64, error) {
	require := require.New(t)

	pledge := power
	if pledge == nil {
		pledge = types.NewBytesAmount(10000)
	}

	// create miner
	msg, err := th.CreateMinerMessage(sn.Addresses[0], nonce, *pledge, RequireRandomPeerID(), types.NewZeroAttoFIL())
	require.NoError(err)
	b := requireMineOnce(ctx, t, cm, lastBlock, rewardAddress, mockSign(sn, msg))
	nonce++

	minerAddr, err := types.NewAddressFromBytes(b.MessageReceipts[0].Return[0])
	require.NoError(err)

	if power == nil {
		return minerAddr, b, nonce, nil
	}

	// create bid
	msg, err = th.AddBidMessage(sn.Addresses[0], nonce, types.NewZeroAttoFIL(), power)
	require.NoError(err)
	b = requireMineOnce(ctx, t, cm, b, rewardAddress, mockSign(sn, msg))
	nonce++

	bidID := bigIntFromBytes(b.MessageReceipts[0].Return[0])

	// create ask
	msg, err = th.AddAskMessage(minerAddr, sn.Addresses[0], nonce, types.NewZeroAttoFIL(), power)
	require.NoError(err)
	b = requireMineOnce(ctx, t, cm, b, rewardAddress, mockSign(sn, msg))
	nonce++

	askID := bigIntFromBytes(b.MessageReceipts[0].Return[0])

	// create deal
	ref, err := cid.NewPrefixV1(cid.DagCBOR, types.DefaultHashFunction).Sum([]byte("Blah"))
	if err != nil {
		return types.Address{}, nil, 0, err
	}
	msg, err = th.AddDealMessage(sn.Addresses[0], nonce, &askID, &bidID, sn.Addresses[0].Bytes(), ref.Bytes())
	require.NoError(err)
	b = requireMineOnce(ctx, t, cm, b, rewardAddress, mockSign(sn, msg))
	nonce++

	dealID := bigIntFromBytes(b.MessageReceipts[0].Return[0])

	// commit sector for deal (thus adding power to miner and recording in storage market.
	msg, err = th.CommitSectorMessage(minerAddr, sn.Addresses[0], nonce, big.NewInt(0), []byte("commitment"), []uint64{dealID.Uint64()})
	require.NoError(err)
	b = requireMineOnce(ctx, t, cm, b, rewardAddress, mockSign(sn, msg))
	nonce++

	return minerAddr, b, nonce, nil
}

func requireMineOnce(ctx context.Context, t *testing.T, cm *ChainManager, lastBlock *types.Block, rewardAddress types.Address, msg *types.SignedMessage) *types.Block {
	require := require.New(t)

	st, err := state.LoadStateTree(ctx, cm.cstore, lastBlock.StateRoot, builtin.Actors)
	vms := vm.NewStorageMap(cm.Blockstore)
	require.NoError(err)

	b := MkChild([]*types.Block{lastBlock}, lastBlock.StateRoot, 0)
	b.Miner = rewardAddress
	if msg != nil {
		b.Messages = append(b.Messages, msg)
	}
	results, err := cm.blockProcessor(ctx, b, st, vms)
	require.NoError(err)
	for _, r := range results {
		b.MessageReceipts = append(b.MessageReceipts, r.Receipt)
	}
	newStateRoot, err := st.Flush(ctx)
	require.NoError(err)

	b.StateRoot = newStateRoot
	_, err = cm.ProcessNewBlock(ctx, b)
	require.NoError(err)

	return b
}

func mockSign(sn types.MockSigner, msg *types.Message) *types.SignedMessage {
	return MustSign(sn, msg)[0]
}

func bigIntFromBytes(i []byte) big.Int {
	var out big.Int
	out.SetBytes(i)
	return out
}
