package chain

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"sync"
	"testing"

	"github.com/ipld/go-car"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"golang.org/x/xerrors"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/venus/fixtures/asset"
	"github.com/filecoin-project/venus/pkg/chainsync/exchange"
	"github.com/filecoin-project/venus/pkg/clock"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/repo"
	emptycid "github.com/filecoin-project/venus/pkg/testhelpers/empty_cid"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/util"
)

// Builder builds fake chains and acts as a provider and fetcher for the chain thus generated.
// All blocks are unique (even if they share parents) and form valid chains of parents and heights,
// but do not carry valid tickets. Each block contributes a weight of 1.
// state root CIDs are computed by an abstract StateBuilder. The default FakeStateBuilder produces
// state CIDs that are distinct but not CIDs of any real state tree. A more sophisticated
// builder could actually apply the messages to a state tree (not yet implemented).
// The builder is deterministic: two builders receiving the same sequence of calls will produce
// exactly the same chain.
type Builder struct {
	t            *testing.T
	genesis      *types.TipSet
	store        *Store
	minerAddress address.Address
	stateBuilder StateBuilder
	stamper      TimeStamper
	repo         repo.Repo
	bs           blockstore.Blockstore
	cstore       cbor.IpldStore
	mstore       *MessageStore
	seq          uint64 // For unique tickets
	eval         *FakeStateEvaluator

	// Cache of the state root CID computed for each tipset key.
	tipStateCids map[string]cid.Cid

	stmgr   IStmgr
	evaLock sync.Mutex
}

func (f *Builder) IStmgr() IStmgr {
	f.evaLock.Lock()
	defer f.evaLock.Unlock()
	return f.stmgr
}

func (f *Builder) FakeStateEvaluator() *FakeStateEvaluator {
	f.evaLock.Lock()
	defer f.evaLock.Unlock()

	if f.eval != nil {
		return f.eval
	}
	f.eval = &FakeStateEvaluator{
		ChainStore:   f.store,
		MessageStore: f.mstore,
		ChsWorkingOn: make(map[types.TipSetKey]chan struct{}, 1),
	}
	return f.eval
}

func (f *Builder) LoadTipSetMessage(ctx context.Context, ts *types.TipSet) ([]types.BlockMessagesInfo, error) {
	// gather message
	applied := make(map[address.Address]uint64)
	selectMsg := func(m *types.UnsignedMessage) (bool, error) {
		// The first match for a sender is guaranteed to have correct nonce -- the block isn't valid otherwise
		if _, ok := applied[m.From]; !ok {
			applied[m.From] = m.Nonce
		}

		if applied[m.From] != m.Nonce {
			return false, nil
		}

		applied[m.From]++

		return true, nil
	}
	blockMsg := []types.BlockMessagesInfo{}
	for i := 0; i < ts.Len(); i++ {
		blk := ts.At(i)
		secpMsgs, blsMsgs, err := f.LoadMetaMessages(ctx, blk.Messages)
		if err != nil {
			return nil, errors.Wrapf(err, "syncing tip %s failed loading message list %s for block %s", ts.Key(), blk.Messages, blk.Cid())
		}

		var sBlsMsg []types.ChainMsg
		var sSecpMsg []types.ChainMsg
		for _, msg := range blsMsgs {
			b, err := selectMsg(msg)
			if err != nil {
				return nil, xerrors.Errorf("failed to decide whether to select message for block: %w", err)
			}
			if b {
				sBlsMsg = append(sBlsMsg, msg)
			}
		}

		for _, msg := range secpMsgs {
			b, err := selectMsg(&msg.Message)
			if err != nil {
				return nil, xerrors.Errorf("failed to decide whether to select message for block: %w", err)
			}
			if b {
				sSecpMsg = append(sSecpMsg, msg)
			}
		}

		blockMsg = append(blockMsg, types.BlockMessagesInfo{
			BlsMessages:   sBlsMsg,
			SecpkMessages: sSecpMsg,
			Block:         blk,
		})
	}

	return blockMsg, nil
}

func (f *Builder) Cstore() cbor.IpldStore {
	return f.cstore
}

func (f *Builder) Genesis() *types.TipSet {
	return f.genesis
}

func (f *Builder) Mstore() *MessageStore {
	return f.mstore
}

func (f *Builder) BlockStore() blockstore.Blockstore {
	return f.bs
}

func (f *Builder) Repo() repo.Repo {
	return f.repo
}

func (f *Builder) Store() *Store {
	return f.store
}

func (f *Builder) RemovePeer(peer peer.ID) {}

var _ BlockProvider = (*Builder)(nil)
var _ TipSetProvider = (*Builder)(nil)
var _ MessageProvider = (*Builder)(nil)

type fakeStmgr struct {
	cs  *Store
	eva *FakeStateEvaluator
}

func (f *fakeStmgr) GetActorAt(ctx context.Context, a address.Address, set *types.TipSet) (*types.Actor, error) {
	return f.cs.GetActorAt(ctx, set, a)
}

func (f *fakeStmgr) RunStateTransition(ctx context.Context, set *types.TipSet) (root cid.Cid, receipts cid.Cid, err error) {
	return f.eva.RunStateTransition(ctx, set)
}

var _ IStmgr = &fakeStmgr{}

// NewBuilder builds a new chain faker with default fake state building.
func NewBuilder(t *testing.T, miner address.Address) *Builder {
	return NewBuilderWithDeps(t, miner, &FakeStateBuilder{}, &ZeroTimestamper{})
}

// NewBuilderWithDeps builds a new chain faker.
// Blocks will have `miner` set as the miner address, or a default if empty.
func NewBuilderWithDeps(t *testing.T, miner address.Address, sb StateBuilder, stamper TimeStamper) *Builder {
	if miner.Empty() {
		var err error
		miner, err = address.NewSecp256k1Address([]byte("miner"))
		require.NoError(t, err)
	}

	repo := repo.NewInMemoryRepo()
	bs := repo.Datastore()
	ds := repo.ChainDatastore()
	cst := cbor.NewCborStore(bs)

	b := &Builder{
		t:            t,
		minerAddress: miner,
		stateBuilder: sb,
		stamper:      stamper,
		repo:         repo,
		bs:           bs,
		cstore:       cst,
		mstore:       NewMessageStore(bs, config.DefaultForkUpgradeParam),
		tipStateCids: make(map[string]cid.Cid),
	}
	ctx := context.TODO()
	_, err := b.mstore.StoreMessages(ctx, []*types.SignedMessage{}, []*types.UnsignedMessage{})
	require.NoError(t, err)
	_, err = b.mstore.StoreReceipts(ctx, []types.MessageReceipt{})
	require.NoError(t, err)
	// append genesis
	nullState := types.CidFromString(t, "null")
	b.tipStateCids[types.NewTipSetKey().String()] = nullState

	// create a fixed genesis
	b.genesis = b.GeneratorGenesis()
	b.store = NewStore(ds, bs, b.genesis.At(0).Cid(), NewMockCirculatingSupplyCalculator())

	for _, block := range b.genesis.Blocks() {
		// add block to cstore
		_, err := b.cstore.Put(context.TODO(), block)
		require.NoError(t, err)
	}

	stateRoot, receiptRoot := b.genesis.Blocks()[0].ParentStateRoot, b.genesis.Blocks()[0].ParentMessageReceipts

	b.tipStateCids[b.genesis.Key().String()] = stateRoot
	require.NoError(t, err)
	tipsetMeta := &TipSetMetadata{
		TipSetStateRoot: stateRoot,
		TipSet:          b.genesis,
		TipSetReceipts:  receiptRoot,
	}
	require.NoError(t, b.store.PutTipSetMetadata(context.TODO(), tipsetMeta))
	err = b.store.SetHead(context.TODO(), b.genesis)
	require.NoError(t, err)

	b.stmgr = &fakeStmgr{cs: b.store, eva: b.FakeStateEvaluator()}

	return b
}

// AppendBlockOnBlocks creates and returns a new block child of `parents`, with no messages.
func (f *Builder) AppendBlockOnBlocks(parents ...*types.BlockHeader) *types.BlockHeader {
	var tip *types.TipSet
	if len(parents) > 0 {
		tip = types.RequireNewTipSet(f.t, parents...)
	}
	return f.AppendBlockOn(tip)
}

// AppendBlockOn creates and returns a new block child of `parent`, with no messages.
func (f *Builder) AppendBlockOn(parent *types.TipSet) *types.BlockHeader {
	return f.Build(parent, 1, nil).At(0)
}

// AppendOn creates and returns a new `width`-block tipset child of `parents`, with no messages.
func (f *Builder) AppendOn(parent *types.TipSet, width int) *types.TipSet {
	return f.Build(parent, width, nil)
}

func (f *Builder) FlushHead(ctx context.Context) error {
	_, _, e := f.FakeStateEvaluator().RunStateTransition(ctx, f.store.GetHead())
	return e
}

// AppendManyBlocksOnBlocks appends `height` blocks to the chain.
func (f *Builder) AppendManyBlocksOnBlocks(height int, parents ...*types.BlockHeader) *types.BlockHeader {
	var tip *types.TipSet
	if len(parents) > 0 {
		tip = types.RequireNewTipSet(f.t, parents...)
	}
	return f.BuildManyOn(height, tip, nil).At(0)
}

// AppendManyBlocksOn appends `height` blocks to the chain.
func (f *Builder) AppendManyBlocksOn(height int, parent *types.TipSet) *types.BlockHeader {
	return f.BuildManyOn(height, parent, nil).At(0)
}

// AppendManyOn appends `height` tipsets to the chain.
func (f *Builder) AppendManyOn(height int, parent *types.TipSet) *types.TipSet {
	return f.BuildManyOn(height, parent, nil)
}

// BuildOnBlock creates and returns a new block child of singleton tipset `parent`. See Build.
func (f *Builder) BuildOnBlock(parent *types.BlockHeader, build func(b *BlockBuilder)) *types.BlockHeader {
	var tip *types.TipSet
	if parent != nil {
		tip = types.RequireNewTipSet(f.t, parent)
	}
	return f.BuildOneOn(tip, build).At(0)
}

// BuildOneOn creates and returns a new single-block tipset child of `parent`.
func (f *Builder) BuildOneOn(parent *types.TipSet, build func(b *BlockBuilder)) *types.TipSet {
	return f.Build(parent, 1, singleBuilder(build))
}

// BuildOn creates and returns a new `width` block tipset child of `parent`.
func (f *Builder) BuildOn(parent *types.TipSet, width int, build func(b *BlockBuilder, i int)) *types.TipSet {
	return f.Build(parent, width, build)
}

// BuildManyOn builds a chain by invoking Build `height` times.
func (f *Builder) BuildManyOn(height int, parent *types.TipSet, build func(b *BlockBuilder)) *types.TipSet {
	require.True(f.t, height > 0, "")
	for i := 0; i < height; i++ {
		parent = f.Build(parent, 1, singleBuilder(build))
	}
	return parent
}

// Build creates and returns a new tipset child of `parent`.
// The tipset carries `width` > 0 blocks with the same height and parents, but different tickets.
// Note: the blocks will all have the same miner, which is unrealistic and forbidden by consensus;
// generalise this to random miner addresses when that is rejected by the syncer.
// The `build` function is invoked to modify the block before it is stored.
func (f *Builder) Build(parent *types.TipSet, width int, build func(b *BlockBuilder, i int)) *types.TipSet {
	tip := f.BuildOrphaTipset(parent, width, build)

	for _, block := range tip.Blocks() {
		// add block to cstore
		_, err := f.cstore.Put(context.TODO(), block)
		require.NoError(f.t, err)
	}

	// Compute and remember state for the tipset.
	stateRoot, _ := f.ComputeState(tip)
	f.tipStateCids[tip.Key().String()] = stateRoot
	return tip
}

func (f *Builder) BuildOrphaTipset(parent *types.TipSet, width int, build func(b *BlockBuilder, i int)) *types.TipSet {
	require.True(f.t, width > 0)
	var blocks []*types.BlockHeader
	height := abi.ChainEpoch(0)
	if parent.Defined() {
		var err error
		height = parent.At(0).Height + 1
		require.NoError(f.t, err)
	} else {
		parent = types.UndefTipSet
	}

	parentWeight, err := f.stateBuilder.Weigh(context.TODO(), parent)
	require.NoError(f.t, err)

	emptyBLSSig := crypto.Signature{
		Type: crypto.SigTypeBLS,
		Data: []byte(""),
		// Data: (*bls.Aggregate([]bls.Signature{}))[:],
	}

	for i := 0; i < width; i++ {
		ticket := types.Ticket{}
		ticket.VRFProof = make([]byte, binary.Size(f.seq))
		binary.BigEndian.PutUint64(ticket.VRFProof, f.seq)
		f.seq++

		b := &types.BlockHeader{
			Ticket:                ticket,
			Miner:                 f.minerAddress,
			BeaconEntries:         nil,
			ParentWeight:          parentWeight,
			Parents:               parent.Key(),
			Height:                height,
			Messages:              emptycid.EmptyTxMetaCID,
			ParentMessageReceipts: emptycid.EmptyReceiptsCID,
			BLSAggregate:          &emptyBLSSig,
			// Omitted fields below
			// ParentStateRoot:       stateRoot,
			// EPoStInfo:       ePoStInfo,
			// ForkSignaling:   forkSig,
			Timestamp:     f.stamper.Stamp(height),
			BlockSig:      &crypto.Signature{Type: crypto.SigTypeSecp256k1, Data: []byte{}},
			ElectionProof: &types.ElectionProof{VRFProof: []byte{0x0c, 0x0d}, WinCount: int64(10)},
		}

		if build != nil {
			build(&BlockBuilder{b, f.t, f.mstore}, i)
		}

		// Compute state root for this block.
		ctx := context.Background()
		prevState := f.StateForKey(parent.Key())
		smsgs, umsgs, err := f.mstore.LoadMetaMessages(ctx, b.Messages)
		require.NoError(f.t, err)

		var sBlsMsg []types.ChainMsg
		var sSecpMsg []types.ChainMsg
		for _, m := range umsgs {
			sBlsMsg = append(sBlsMsg, m)
		}

		for _, m := range smsgs {
			sSecpMsg = append(sSecpMsg, m)
		}
		blkMsgInfo := types.BlockMessagesInfo{
			BlsMessages:   sBlsMsg,
			SecpkMessages: sSecpMsg,
			Block:         b,
		}
		stateRootRaw, _, err := f.stateBuilder.ComputeState(prevState, []types.BlockMessagesInfo{blkMsgInfo})
		require.NoError(f.t, err)
		b.ParentStateRoot = stateRootRaw

		blocks = append(blocks, b)
	}
	return types.RequireNewTipSet(f.t, blocks...)
}

// StateForKey loads (or computes) the state root for a tipset key.
func (f *Builder) StateForKey(key types.TipSetKey) cid.Cid {
	state, found := f.tipStateCids[key.String()]
	if found {
		return state
	}
	// No state yet computed for this tip (perhaps because the blocks in it have not previously
	// been considered together as a tipset).
	tip, err := f.GetTipSet(key)
	require.NoError(f.t, err)
	state, _ = f.ComputeState(tip)
	return state
}

// GetBlockstoreValue gets data straight out of the underlying blockstore by cid
func (f *Builder) GetBlockstoreValue(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	return f.bs.Get(c)
}

// ComputeState computes the state for a tipset from its parent state.
func (f *Builder) ComputeState(tip *types.TipSet) (cid.Cid, []types.MessageReceipt) {
	parentKey := tip.Parents()
	// Load the state of the parent tipset and compute the required state (recursively).
	prev := f.StateForKey(parentKey)
	blockMsgInfo := f.tipMessages(tip)
	state, receipt, err := f.stateBuilder.ComputeState(prev, blockMsgInfo)
	require.NoError(f.t, err)
	return state, receipt
}

// tipMessages returns the messages of a tipset.  Each block's messages are
// grouped into a slice and a slice of these slices is returned.
func (f *Builder) tipMessages(tip *types.TipSet) []types.BlockMessagesInfo {
	ctx := context.Background()
	var blockMessageInfos []types.BlockMessagesInfo
	for i := 0; i < tip.Len(); i++ {
		smsgs, blsMsg, err := f.mstore.LoadMetaMessages(ctx, tip.At(i).Messages)
		require.NoError(f.t, err)

		var sBlsMsg []types.ChainMsg
		var sSecpMsg []types.ChainMsg
		for _, m := range blsMsg {
			sBlsMsg = append(sBlsMsg, m)
		}

		for _, m := range smsgs {
			sSecpMsg = append(sSecpMsg, m)
		}
		blockMessageInfos = append(blockMessageInfos, types.BlockMessagesInfo{
			BlsMessages:   sBlsMsg,
			SecpkMessages: sSecpMsg,
			Block:         tip.At(i),
		})
	}
	return blockMessageInfos
}

// Wraps a simple build function in one that also accepts an index, propagating a nil function.
func singleBuilder(build func(b *BlockBuilder)) func(b *BlockBuilder, i int) {
	if build == nil {
		return nil
	}
	return func(b *BlockBuilder, i int) { build(b) }
}

// /// BlockHeader builder /////

// BlockBuilder mutates blocks as they are generated.
type BlockBuilder struct {
	block    *types.BlockHeader
	t        *testing.T
	messages *MessageStore
}

// SetTicket sets the block's ticket.
func (bb *BlockBuilder) SetTicket(raw []byte) {
	bb.block.Ticket = types.Ticket{VRFProof: types.VRFPi(raw)}
}

// SetTimestamp sets the block's timestamp.
func (bb *BlockBuilder) SetTimestamp(timestamp uint64) {
	bb.block.Timestamp = timestamp
}

// IncHeight increments the block's height, implying a number of null blocks before this one
// is mined.
func (bb *BlockBuilder) IncHeight(nullBlocks abi.ChainEpoch) {
	bb.block.Height += nullBlocks
}

// SetBlockSig set a new signature
func (bb *BlockBuilder) SetBlockSig(signature crypto.Signature) {
	bb.block.BlockSig = &signature
}

// AddMessages adds a message & receipt collection to the block.
func (bb *BlockBuilder) AddMessages(secpmsgs []*types.SignedMessage, blsMsgs []*types.UnsignedMessage) {
	ctx := context.Background()

	meta, err := bb.messages.StoreMessages(ctx, secpmsgs, blsMsgs)
	require.NoError(bb.t, err)

	bb.block.Messages = meta
}

// SetStateRoot sets the block's state root.
func (bb *BlockBuilder) SetStateRoot(root cid.Cid) {
	bb.block.ParentStateRoot = root
}

// /// state builder /////

// StateBuilder abstracts the computation of state root CIDs from the chain builder.
type StateBuilder interface {
	ComputeState(prev cid.Cid, blockmsg []types.BlockMessagesInfo) (cid.Cid, []types.MessageReceipt, error)
	Weigh(ctx context.Context, tip *types.TipSet) (big.Int, error)
}

// FakeStateBuilder computes a fake state CID by hashing the CIDs of a block's parents and messages.
type FakeStateBuilder struct {
}

// ComputeState computes a fake state from a previous state root CID and the messages contained
// in list-of-lists of messages in blocks. Note that if there are no messages, the resulting state
// is the same as the input state.
// This differs from the true state transition function in that messages that are duplicated
// between blocks in the tipset are not ignored.
func (FakeStateBuilder) ComputeState(prev cid.Cid, blockmsg []types.BlockMessagesInfo) (cid.Cid, []types.MessageReceipt, error) {
	receipts := []types.MessageReceipt{}

	// Accumulate the cids of the previous state and of all messages in the tipset.
	inputs := []cid.Cid{prev}
	for _, blockMessages := range blockmsg {
		for _, msg := range append(blockMessages.BlsMessages, blockMessages.SecpkMessages...) {
			mCId := msg.Cid()
			inputs = append(inputs, mCId)
			receipts = append(receipts, types.MessageReceipt{
				ExitCode:    0,
				ReturnValue: mCId.Bytes(),
				GasUsed:     3,
			})
		}
	}

	if len(inputs) == 1 {
		// If there are no messages, the state doesn't change!
		return prev, receipts, nil
	}

	root, err := util.MakeCid(inputs)
	if err != nil {
		return cid.Undef, []types.MessageReceipt{}, err
	}
	return root, receipts, nil
}

// Weigh computes a tipset's weight as its parent weight plus one for each block in the tipset.
func (FakeStateBuilder) Weigh(context context.Context, tip *types.TipSet) (big.Int, error) {
	parentWeight := big.Zero()
	if tip.Defined() {
		parentWeight = tip.ParentWeight()
	}

	return big.Add(parentWeight, big.NewInt(int64(tip.Len()))), nil
}

// /// Timestamper /////

// TimeStamper is an object that timestamps blocks
type TimeStamper interface {
	Stamp(abi.ChainEpoch) uint64
}

// ZeroTimestamper writes a default of 0 to the timestamp
type ZeroTimestamper struct{}

// Stamp returns a stamp for the current block
func (zt *ZeroTimestamper) Stamp(height abi.ChainEpoch) uint64 {
	return uint64(0)
}

// ClockTimestamper writes timestamps based on a blocktime and genesis time
type ClockTimestamper struct {
	c clock.ChainEpochClock
}

// NewClockTimestamper makes a new stamper for creating production valid timestamps
func NewClockTimestamper(chainClock clock.ChainEpochClock) *ClockTimestamper {
	return &ClockTimestamper{
		c: chainClock,
	}
}

// Stamp assigns a valid timestamp given genesis time and block time to
// a block of the provided height.
func (ct *ClockTimestamper) Stamp(height abi.ChainEpoch) uint64 {
	startTime := ct.c.StartTimeOfEpoch(height)

	return uint64(startTime.Unix())
}

// /// state evaluator /////

// FakeStateEvaluator is a syncStateEvaluator that delegates to the FakeStateBuilder.
type FakeStateEvaluator struct {
	ChainStore   *Store
	MessageStore *MessageStore
	FakeStateBuilder
	ChsWorkingOn map[types.TipSetKey]chan struct{}
	stLk         sync.Mutex
}

// RunStateTransition delegates to StateBuilder.ComputeState
func (e *FakeStateEvaluator) RunStateTransition(ctx context.Context, ts *types.TipSet) (rootCid cid.Cid, receiptCid cid.Cid, err error) {
	key := ts.Key()
	e.stLk.Lock()
	workingCh, exist := e.ChsWorkingOn[key]

	if exist {
		e.stLk.Unlock()
		select {
		case <-workingCh:
			e.stLk.Lock()
		case <-ctx.Done():
			return cid.Undef, cid.Undef, ctx.Err()
		}
	}
	if m, _ := e.ChainStore.LoadTipsetMetadata(ts); m != nil {
		e.stLk.Unlock()
		return m.TipSetStateRoot, m.TipSetReceipts, nil
	}

	workingCh = make(chan struct{})
	e.ChsWorkingOn[key] = workingCh
	e.stLk.Unlock()

	defer func() {
		e.stLk.Lock()
		delete(e.ChsWorkingOn, key)
		if err == nil {
			_ = e.ChainStore.PutTipSetMetadata(ctx, &TipSetMetadata{
				TipSetStateRoot: rootCid,
				TipSet:          ts,
				TipSetReceipts:  receiptCid,
			})
		}
		e.stLk.Unlock()
		close(workingCh)
	}()

	// gather message
	blockMessageInfo, err := e.MessageStore.LoadTipSetMessage(ctx, ts)
	if err != nil {
		return cid.Undef, cid.Undef, xerrors.Errorf("failed to gather message in tipset %v", err)
	}
	var receipts []types.MessageReceipt
	rootCid, receipts, err = e.ComputeState(ts.At(0).ParentStateRoot, blockMessageInfo)
	if err != nil {
		return cid.Undef, cid.Undef, errors.Wrap(err, "error compute state")
	}

	receiptCid, err = e.MessageStore.StoreReceipts(ctx, receipts)
	if err != nil {
		return cid.Undef, cid.Undef, xerrors.Errorf("failed to save receipt: %v", err)
	}

	return rootCid, receiptCid, nil
}

func (e *FakeStateEvaluator) ValidateFullBlock(ctx context.Context, blk *types.BlockHeader) error {
	parent, err := e.ChainStore.GetTipSet(blk.Parents)
	if err != nil {
		return err
	}

	root, receipts, err := e.RunStateTransition(ctx, parent)
	if err != nil {
		return err
	}

	return e.ChainStore.PutTipSetMetadata(ctx, &TipSetMetadata{
		TipSetStateRoot: root,
		TipSet:          parent,
		TipSetReceipts:  receipts,
	})
}

// /// Chain selector /////

// FakeChainSelector is a syncChainSelector that delegates to the FakeStateBuilder
type FakeChainSelector struct {
	FakeStateBuilder
}

// IsHeavier compares chains weighed with StateBuilder.Weigh.
func (e *FakeChainSelector) IsHeavier(ctx context.Context, a, b *types.TipSet) (bool, error) {
	aw, err := e.Weigh(ctx, a)
	if err != nil {
		return false, err
	}
	bw, err := e.Weigh(ctx, b)
	if err != nil {
		return false, err
	}
	return aw.GreaterThan(bw), nil
}

// Weight delegates to the statebuilder
func (e *FakeChainSelector) Weight(ctx context.Context, ts *types.TipSet) (big.Int, error) {
	return e.Weigh(ctx, ts)
}

// /// Interface and accessor implementations /////

// GetBlock returns the block identified by `c`.
func (f *Builder) GetBlock(ctx context.Context, c cid.Cid) (*types.BlockHeader, error) {
	var block types.BlockHeader
	if err := f.cstore.Get(ctx, c, &block); err != nil {
		return nil, err
	}
	return &block, nil
}

// GetBlocks returns the blocks identified by `cids`.
func (f *Builder) GetBlocksByIds(ctx context.Context, cids []cid.Cid) ([]*types.BlockHeader, error) {
	ret := make([]*types.BlockHeader, len(cids))
	for i, c := range cids {
		var block types.BlockHeader
		if err := f.cstore.Get(ctx, c, &block); err != nil {
			return nil, err
		}
		ret[i] = &block
	}
	return ret, nil
}

// GetTipSet returns the tipset identified by `key`.
func (f *Builder) GetTipSet(key types.TipSetKey) (*types.TipSet, error) {
	ctx := context.Background()
	var blocks []*types.BlockHeader
	for _, bid := range key.Cids() {
		var blk types.BlockHeader
		if err := f.cstore.Get(ctx, bid, &blk); err != nil {
			return nil, fmt.Errorf("no block %s", bid)
		}
		blocks = append(blocks, &blk)
	}
	return types.NewTipSet(blocks...)
}

// FetchTipSets fetchs the tipset at `tsKey` from the fetchers blockStore backed by the Builder.
func (f *Builder) FetchTipSets(ctx context.Context, key types.TipSetKey, from peer.ID, done func(t *types.TipSet) (bool, error)) ([]*types.TipSet, error) {
	var tips []*types.TipSet
	for {
		tip, err := f.GetTipSet(key)
		if err != nil {
			return nil, err
		}
		tips = append(tips, tip)
		ok, err := done(tip)
		if err != nil {
			return nil, err
		}
		if ok {
			break
		}
		key = tip.Parents()
	}
	return tips, nil
}

// FetchTipSetHeaders fetchs the tipset at `tsKey` from the fetchers blockStore backed by the Builder.
func (f *Builder) FetchTipSetHeaders(ctx context.Context, key types.TipSetKey, from peer.ID, done func(t *types.TipSet) (bool, error)) ([]*types.TipSet, error) {
	return f.FetchTipSets(ctx, key, from, done)
}

// GetTipSetStateRoot returns the state root that was computed for a tipset.
func (f *Builder) GetTipSetStateRoot(key types.TipSetKey) (cid.Cid, error) {
	found, ok := f.tipStateCids[key.String()]
	if !ok {
		return cid.Undef, errors.Errorf("no state for %s", key)
	}
	return found, nil
}

func (f *Builder) GetTipSetByHeight(ctx context.Context, ts *types.TipSet, h abi.ChainEpoch, prev bool) (*types.TipSet, error) {
	if !ts.Defined() {
		return ts, nil
	}
	if epoch := ts.Height(); epoch == h {
		return ts, nil
	}

	for {
		ts = f.RequireTipSet(ts.Parents())
		height := ts.Height()
		if height >= 0 && height == h {
			return ts, nil
		} else if height < h {
			return ts, nil
		}
	}
}

// RequireTipSet returns a tipset by key, which must exist.
func (f *Builder) RequireTipSet(key types.TipSetKey) *types.TipSet {
	tip, err := f.GetTipSet(key)
	require.NoError(f.t, err)
	return tip
}

// RequireTipSets returns a chain of tipsets from key, which must exist and be long enough.
func (f *Builder) RequireTipSets(head types.TipSetKey, count int) []*types.TipSet {
	var tips []*types.TipSet
	for i := 0; i < count; i++ {
		tip := f.RequireTipSet(head)
		tips = append(tips, tip)
		head = tip.Parents()
	}
	return tips
}

// LoadReceipts returns the message collections tracked by the builder.
func (f *Builder) LoadReceipts(ctx context.Context, c cid.Cid) ([]types.MessageReceipt, error) {
	return f.mstore.LoadReceipts(ctx, c)
}

// LoadTxMeta returns the tx meta wrapper tracked by the builder.
func (f *Builder) LoadTxMeta(ctx context.Context, metaCid cid.Cid) (types.TxMeta, error) {
	return f.mstore.LoadTxMeta(ctx, metaCid)
}

// StoreReceipts stores message receipts and returns a commitment.
func (f *Builder) StoreReceipts(ctx context.Context, receipts []types.MessageReceipt) (cid.Cid, error) {
	return f.mstore.StoreReceipts(ctx, receipts)
}

// StoreTxMeta stores a tx meta
func (f *Builder) StoreTxMeta(ctx context.Context, meta types.TxMeta) (cid.Cid, error) {
	return f.mstore.StoreTxMeta(ctx, meta)
}

func (f *Builder) LoadUnsignedMessagesFromCids(blsCids []cid.Cid) ([]*types.UnsignedMessage, error) {
	return f.mstore.LoadUnsignedMessagesFromCids(blsCids)
}

func (f *Builder) LoadSignedMessagesFromCids(secpCids []cid.Cid) ([]*types.SignedMessage, error) {
	return f.mstore.LoadSignedMessagesFromCids(secpCids)
}

// LoadMessages returns the message collections tracked by the builder.
func (f *Builder) LoadMetaMessages(ctx context.Context, metaCid cid.Cid) ([]*types.SignedMessage, []*types.UnsignedMessage, error) {
	return f.mstore.LoadMetaMessages(ctx, metaCid)
}

func (f *Builder) ReadMsgMetaCids(ctx context.Context, metaCid cid.Cid) ([]cid.Cid, []cid.Cid, error) {
	return f.mstore.ReadMsgMetaCids(ctx, metaCid)
}

// /// exchange  /////
func (f *Builder) GetBlocks(ctx context.Context, tsk types.TipSetKey, count int) ([]*types.TipSet, error) {
	ts, err := f.GetTipSet(tsk)
	if err != nil {
		return nil, err
	}
	result := []*types.TipSet{ts}
	for i := 1; i < count; i++ {
		if ts.Height() == 0 {
			break
		}
		ts, err = f.GetTipSet(ts.Parents())
		if err != nil {
			return nil, err
		}
		result = append(result, ts)
	}
	return result, nil
}

func (f *Builder) GetChainMessages(ctx context.Context, tipsets []*types.TipSet) ([]*exchange.CompactedMessages, error) {
	result := []*exchange.CompactedMessages{}
	for _, ts := range tipsets {
		bmsgs, bmincl, smsgs, smincl, err := exchange.GatherMessages(f, f.mstore, ts)
		if err != nil {
			return nil, err
		}
		compactMsg := &exchange.CompactedMessages{}
		compactMsg.Bls = bmsgs
		compactMsg.BlsIncludes = bmincl
		compactMsg.Secpk = smsgs
		compactMsg.SecpkIncludes = smincl
		result = append(result, compactMsg)
	}
	return result, nil
}

func (f *Builder) GetFullTipSet(ctx context.Context, peer []peer.ID, tsk types.TipSetKey) (*types.FullTipSet, error) {
	panic("implement me")
}

func (f *Builder) AddPeer(peer peer.ID) {}

func (f *Builder) GeneratorGenesis() *types.TipSet {
	b, err := asset.Asset("fixtures/_assets/car/calibnet.car")
	require.NoError(f.t, err)
	source := ioutil.NopCloser(bytes.NewReader(b))

	ch, err := car.LoadCar(f.bs, source)
	require.NoError(f.t, err)

	// need to check if we are being handed a car file with a single genesis block or an entire chain.
	bsBlk, err := f.bs.Get(ch.Roots[0])
	require.NoError(f.t, err)

	cur, err := types.DecodeBlock(bsBlk.RawData())
	require.NoError(f.t, err)

	ts, err := types.NewTipSet(cur)
	require.NoError(f.t, err)

	return ts
}
