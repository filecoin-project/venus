package chain

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"

	ds "github.com/ipfs/go-datastore"
	syncds "github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/go-hamt-ipld"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	bls "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

// Builder builds fake chains and acts as a provider and fetcher for the chain thus generated.
// All blocks are unique (even if they share parents) and form valid chains of parents and heights,
// but do not carry valid tickets. Each block contributes a weight of 1.
// State root CIDs are computed by an abstract StateBuilder. The default FakeStateBuilder produces
// state CIDs that are distinct but not CIDs of any real state tree. A more sophisticated
// builder could actually apply the messages to a state tree (not yet implemented).
// The builder is deterministic: two builders receiving the same sequence of calls will produce
// exactly the same chain.
type Builder struct {
	t            *testing.T
	minerAddress address.Address
	stateBuilder StateBuilder
	stamper      TimeStamper
	bs           blockstore.Blockstore
	cstore       *hamt.CborIpldStore
	messages     *MessageStore
	seq          uint64 // For unique tickets

	// Cache of the state root CID computed for each tipset key.
	tipStateCids map[string]cid.Cid
}

var _ BlockProvider = (*Builder)(nil)
var _ TipSetProvider = (*Builder)(nil)
var _ MessageProvider = (*Builder)(nil)

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

	bs := blockstore.NewBlockstore(syncds.MutexWrap(ds.NewMapDatastore()))
	b := &Builder{
		t:            t,
		minerAddress: miner,
		stateBuilder: sb,
		stamper:      stamper,
		bs:           bs,
		cstore:       hamt.CSTFromBstore(bs),
		messages:     NewMessageStore(bs),
		tipStateCids: make(map[string]cid.Cid),
	}

	ctx := context.TODO()
	_, err := b.messages.StoreMessages(ctx, []*types.SignedMessage{}, []*types.UnsignedMessage{})
	require.NoError(t, err)
	_, err = b.messages.StoreReceipts(ctx, []*types.MessageReceipt{})
	require.NoError(t, err)

	nullState := types.CidFromString(t, "null")
	b.tipStateCids[block.NewTipSetKey().String()] = nullState
	return b
}

// NewGenesis creates and returns a tipset of one block with no parents.
func (f *Builder) NewGenesis() block.TipSet {
	return th.RequireNewTipSet(f.t, f.AppendBlockOn(block.UndefTipSet))
}

// AppendBlockOnBlocks creates and returns a new block child of `parents`, with no messages.
func (f *Builder) AppendBlockOnBlocks(parents ...*block.Block) *block.Block {
	tip := block.UndefTipSet
	if len(parents) > 0 {
		tip = th.RequireNewTipSet(f.t, parents...)
	}
	return f.AppendBlockOn(tip)
}

// AppendBlockOn creates and returns a new block child of `parent`, with no messages.
func (f *Builder) AppendBlockOn(parent block.TipSet) *block.Block {
	return f.Build(parent, 1, nil).At(0)
}

// AppendOn creates and returns a new `width`-block tipset child of `parents`, with no messages.
func (f *Builder) AppendOn(parent block.TipSet, width int) block.TipSet {
	return f.Build(parent, width, nil)
}

// AppendManyBlocksOnBlocks appends `height` blocks to the chain.
func (f *Builder) AppendManyBlocksOnBlocks(height int, parents ...*block.Block) *block.Block {
	tip := block.UndefTipSet
	if len(parents) > 0 {
		tip = th.RequireNewTipSet(f.t, parents...)
	}
	return f.BuildManyOn(height, tip, nil).At(0)
}

// AppendManyBlocksOn appends `height` blocks to the chain.
func (f *Builder) AppendManyBlocksOn(height int, parent block.TipSet) *block.Block {
	return f.BuildManyOn(height, parent, nil).At(0)
}

// AppendManyOn appends `height` tipsets to the chain.
func (f *Builder) AppendManyOn(height int, parent block.TipSet) block.TipSet {
	return f.BuildManyOn(height, parent, nil)
}

// BuildOnBlock creates and returns a new block child of singleton tipset `parent`. See Build.
func (f *Builder) BuildOnBlock(parent *block.Block, build func(b *BlockBuilder)) *block.Block {
	tip := block.UndefTipSet
	if parent != nil {
		tip = th.RequireNewTipSet(f.t, parent)
	}
	return f.BuildOneOn(tip, build).At(0)
}

// BuildOneOn creates and returns a new single-block tipset child of `parent`.
func (f *Builder) BuildOneOn(parent block.TipSet, build func(b *BlockBuilder)) block.TipSet {
	return f.Build(parent, 1, singleBuilder(build))
}

// BuildOn creates and returns a new `width` block tipset child of `parent`.
func (f *Builder) BuildOn(parent block.TipSet, width int, build func(b *BlockBuilder, i int)) block.TipSet {
	return f.Build(parent, width, build)
}

// BuildManyOn builds a chain by invoking Build `height` times.
func (f *Builder) BuildManyOn(height int, parent block.TipSet, build func(b *BlockBuilder)) block.TipSet {
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
func (f *Builder) Build(parent block.TipSet, width int, build func(b *BlockBuilder, i int)) block.TipSet {
	require.True(f.t, width > 0)
	var blocks []*block.Block

	height := types.Uint64(0)
	grandparentKey := block.NewTipSetKey()
	if parent.Defined() {
		var err error
		height = parent.At(0).Height + 1
		grandparentKey, err = parent.Parents()
		require.NoError(f.t, err)
	}

	parentWeight, err := f.stateBuilder.Weigh(parent, f.StateForKey(grandparentKey))
	require.NoError(f.t, err)

	emptyBLSSig := (*bls.Aggregate([]bls.Signature{}))[:]

	for i := 0; i < width; i++ {
		ticket := block.Ticket{}
		ticket.VRFProof = block.VRFPi(make([]byte, binary.Size(f.seq)))
		binary.BigEndian.PutUint64(ticket.VRFProof, f.seq)
		f.seq++

		b := &block.Block{
			Ticket:          ticket,
			Miner:           f.minerAddress,
			ParentWeight:    types.Uint64(parentWeight),
			Parents:         parent.Key(),
			Height:          height,
			Messages:        types.TxMeta{SecpRoot: types.EmptyMessagesCID, BLSRoot: types.EmptyMessagesCID},
			MessageReceipts: types.EmptyReceiptsCID,
			BLSAggregateSig: emptyBLSSig,
			// Omitted fields below
			//StateRoot:       stateRoot,
			//Proof            PoStProof
			Timestamp: f.stamper.Stamp(uint64(height)),
		}
		// Nonce intentionally omitted as it will go away.

		if build != nil {
			build(&BlockBuilder{b, f.t, f.messages}, i)
		}

		// Compute state root for this block.
		ctx := context.Background()
		prevState := f.StateForKey(parent.Key())
		smsgs, umsgs, err := f.messages.LoadMessages(ctx, b.Messages)
		require.NoError(f.t, err)
		b.StateRoot, _, err = f.stateBuilder.ComputeState(prevState, [][]*types.UnsignedMessage{umsgs}, [][]*types.SignedMessage{smsgs})
		require.NoError(f.t, err)

		// add block to cstore
		_, err = f.cstore.Put(ctx, b)
		require.NoError(f.t, err)
		blocks = append(blocks, b)
	}
	tip := th.RequireNewTipSet(f.t, blocks...)
	// Compute and remember state for the tipset.
	f.tipStateCids[tip.Key().String()] = f.ComputeState(tip)
	return tip
}

// StateForKey loads (or computes) the state root for a tipset key.
func (f *Builder) StateForKey(key block.TipSetKey) cid.Cid {
	state, found := f.tipStateCids[key.String()]
	if found {
		return state
	}
	// No state yet computed for this tip (perhaps because the blocks in it have not previously
	// been considered together as a tipset).
	tip, err := f.GetTipSet(key)
	require.NoError(f.t, err)
	return f.ComputeState(tip)
}

// GetBlockstoreValue gets data straight out of the underlying blockstore by cid
func (f *Builder) GetBlockstoreValue(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	return f.bs.Get(c)
}

// ComputeState computes the state for a tipset from its parent state.
func (f *Builder) ComputeState(tip block.TipSet) cid.Cid {
	parentKey, err := tip.Parents()
	require.NoError(f.t, err)
	// Load the state of the parent tipset and compute the required state (recursively).
	prev := f.StateForKey(parentKey)
	state, _, err := f.stateBuilder.ComputeState(prev, [][]*types.UnsignedMessage{}, f.tipMessages(tip))
	require.NoError(f.t, err)
	return state
}

// tipMessages returns the messages of a tipset.  Each block's messages are
// grouped into a slice and a slice of these slices is returned.
func (f *Builder) tipMessages(tip block.TipSet) [][]*types.SignedMessage {
	ctx := context.Background()
	var msgs [][]*types.SignedMessage
	for i := 0; i < tip.Len(); i++ {
		smsgs, _, err := f.messages.LoadMessages(ctx, tip.At(i).Messages)
		require.NoError(f.t, err)
		msgs = append(msgs, smsgs)
	}
	return msgs
}

// Wraps a simple build function in one that also accepts an index, propagating a nil function.
func singleBuilder(build func(b *BlockBuilder)) func(b *BlockBuilder, i int) {
	if build == nil {
		return nil
	}
	return func(b *BlockBuilder, i int) { build(b) }
}

///// Block builder /////

// BlockBuilder mutates blocks as they are generated.
type BlockBuilder struct {
	block    *block.Block
	t        *testing.T
	messages *MessageStore
}

// SetTicket sets the block's ticket.
func (bb *BlockBuilder) SetTicket(raw []byte) {
	bb.block.Ticket = block.Ticket{VRFProof: block.VRFPi(raw)}
}

// SetTimestamp sets the block's timestamp.
func (bb *BlockBuilder) SetTimestamp(timestamp types.Uint64) {
	bb.block.Timestamp = timestamp
}

// IncHeight increments the block's height, implying a number of null blocks before this one
// is mined.
func (bb *BlockBuilder) IncHeight(nullBlocks types.Uint64) {
	bb.block.Height += nullBlocks
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
	bb.block.StateRoot = root
}

///// State builder /////

// StateBuilder abstracts the computation of state root CIDs from the chain builder.
type StateBuilder interface {
	ComputeState(prev cid.Cid, blsMessages [][]*types.UnsignedMessage, secpMessages [][]*types.SignedMessage) (cid.Cid, []*types.MessageReceipt, error)
	Weigh(tip block.TipSet, state cid.Cid) (uint64, error)
}

// FakeStateBuilder computes a fake state CID by hashing the CIDs of a block's parents and messages.
type FakeStateBuilder struct {
}

// ComputeState computes a fake state from a previous state root CID and the messages contained
// in list-of-lists of messages in blocks. Note that if there are no messages, the resulting state
// is the same as the input state.
// This differs from the true state transition function in that messages that are duplicated
// between blocks in the tipset are not ignored.
func (FakeStateBuilder) ComputeState(prev cid.Cid, blsMessages [][]*types.UnsignedMessage, secpMessages [][]*types.SignedMessage) (cid.Cid, []*types.MessageReceipt, error) {
	receipts := []*types.MessageReceipt{}

	// Accumulate the cids of the previous state and of all messages in the tipset.
	inputs := []cid.Cid{prev}
	for _, blockMessages := range blsMessages {
		for _, msg := range blockMessages {
			mCId, err := msg.Cid()
			if err != nil {
				return cid.Undef, []*types.MessageReceipt{}, err
			}
			inputs = append(inputs, mCId)
			receipts = append(receipts, &types.MessageReceipt{
				ExitCode:   0,
				Return:     [][]byte{mCId.Bytes()},
				GasAttoFIL: types.NewGasPrice(3),
			})
		}
	}
	for _, blockMessages := range secpMessages {
		for _, msg := range blockMessages {
			mCId, err := msg.Cid()
			if err != nil {
				return cid.Undef, []*types.MessageReceipt{}, err
			}
			inputs = append(inputs, mCId)
			receipts = append(receipts, &types.MessageReceipt{
				ExitCode:   0,
				Return:     [][]byte{mCId.Bytes()},
				GasAttoFIL: types.NewGasPrice(3),
			})
		}
	}

	if len(inputs) == 1 {
		// If there are no messages, the state doesn't change!
		return prev, receipts, nil
	}

	root, err := makeCid(inputs)
	if err != nil {
		return cid.Undef, []*types.MessageReceipt{}, err
	}
	return root, receipts, nil
}

// Weigh computes a tipset's weight as its parent weight plus one for each block in the tipset.
func (FakeStateBuilder) Weigh(tip block.TipSet, state cid.Cid) (uint64, error) {
	parentWeight := uint64(0)
	if tip.Defined() {
		var err error
		parentWeight, err = tip.ParentWeight()
		if err != nil {
			return 0, err
		}
	}
	return parentWeight + uint64(tip.Len()), nil
}

///// Timestamper /////

// TimeStamper is an object that timestamps blocks
type TimeStamper interface {
	Stamp(uint64) types.Uint64
}

// ZeroTimestamper writes a default of 0 to the timestamp
type ZeroTimestamper struct{}

// Stamp returns a stamp for the current block
func (zt *ZeroTimestamper) Stamp(height uint64) types.Uint64 {
	return types.Uint64(0)
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
func (ct *ClockTimestamper) Stamp(height uint64) types.Uint64 {
	startTime := ct.c.StartTimeOfEpoch(types.NewBlockHeight(height))

	timestamp := uint64(startTime.Unix())
	return types.Uint64(timestamp)
}

///// State evaluator /////

// FakeStateEvaluator is a syncStateEvaluator that delegates to the FakeStateBuilder.
type FakeStateEvaluator struct {
	FakeStateBuilder
}

// RunStateTransition delegates to StateBuilder.ComputeState.
func (e *FakeStateEvaluator) RunStateTransition(ctx context.Context, tip block.TipSet, blsMessages [][]*types.UnsignedMessage, secpMessages [][]*types.SignedMessage, ancestors []block.TipSet, parentWeight uint64, stateID cid.Cid, receiptCid cid.Cid) (cid.Cid, []*types.MessageReceipt, error) {
	return e.ComputeState(stateID, blsMessages, secpMessages)
}

// ValidateSemantic is a stub that always returns no error
func (e *FakeStateEvaluator) ValidateSemantic(_ context.Context, _ *block.Block, _ block.TipSet) error {
	return nil
}

///// Chain selector /////

// FakeChainSelector is a syncChainSelector that delegates to the FakeStateBuilder
type FakeChainSelector struct {
	FakeStateBuilder
}

// IsHeavier compares chains weighed with StateBuilder.Weigh.
func (e *FakeChainSelector) IsHeavier(ctx context.Context, a, b block.TipSet, aStateID, bStateID cid.Cid) (bool, error) {
	aw, err := e.Weigh(a, aStateID)
	if err != nil {
		return false, err
	}
	bw, err := e.Weigh(b, bStateID)
	if err != nil {
		return false, err
	}
	return aw > bw, nil
}

// Weight delegates to the statebuilder
func (e *FakeChainSelector) Weight(ctx context.Context, ts block.TipSet, stID cid.Cid) (uint64, error) {
	return e.Weigh(ts, stID)
}

///// Interface and accessor implementations /////

// GetBlock returns the block identified by `c`.
func (f *Builder) GetBlock(ctx context.Context, c cid.Cid) (*block.Block, error) {
	var block block.Block
	if err := f.cstore.Get(ctx, c, &block); err != nil {
		return nil, err
	}
	return &block, nil
}

// GetBlocks returns the blocks identified by `cids`.
func (f *Builder) GetBlocks(ctx context.Context, cids []cid.Cid) ([]*block.Block, error) {
	ret := make([]*block.Block, len(cids))
	for i, c := range cids {
		var block block.Block
		if err := f.cstore.Get(ctx, c, &block); err != nil {
			return nil, err
		}
		ret[i] = &block
	}
	return ret, nil
}

// GetTipSet returns the tipset identified by `key`.
func (f *Builder) GetTipSet(key block.TipSetKey) (block.TipSet, error) {
	ctx := context.Background()
	var blocks []*block.Block
	for it := key.Iter(); !it.Complete(); it.Next() {
		var blk block.Block
		if err := f.cstore.Get(ctx, it.Value(), &blk); err != nil {
			return block.UndefTipSet, fmt.Errorf("no block %s", it.Value())
		}
		blocks = append(blocks, &blk)
	}
	return block.NewTipSet(blocks...)
}

// FetchTipSets fetchs the tipset at `tsKey` from the fetchers blockStore backed by the Builder.
func (f *Builder) FetchTipSets(ctx context.Context, key block.TipSetKey, from peer.ID, done func(t block.TipSet) (bool, error)) ([]block.TipSet, error) {
	var tips []block.TipSet
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
		key, err = tip.Parents()
		if err != nil {
			return nil, err
		}
	}
	return tips, nil
}

// FetchTipSetHeaders fetchs the tipset at `tsKey` from the fetchers blockStore backed by the Builder.
func (f *Builder) FetchTipSetHeaders(ctx context.Context, key block.TipSetKey, from peer.ID, done func(t block.TipSet) (bool, error)) ([]block.TipSet, error) {
	return f.FetchTipSets(ctx, key, from, done)
}

// GetTipSetStateRoot returns the state root that was computed for a tipset.
func (f *Builder) GetTipSetStateRoot(key block.TipSetKey) (cid.Cid, error) {
	found, ok := f.tipStateCids[key.String()]
	if !ok {
		return cid.Undef, errors.Errorf("no state for %s", key)
	}
	return found, nil
}

// RequireTipSet returns a tipset by key, which must exist.
func (f *Builder) RequireTipSet(key block.TipSetKey) block.TipSet {
	tip, err := f.GetTipSet(key)
	require.NoError(f.t, err)
	return tip
}

// RequireTipSets returns a chain of tipsets from key, which must exist and be long enough.
func (f *Builder) RequireTipSets(head block.TipSetKey, count int) []block.TipSet {
	var tips []block.TipSet
	var err error
	for i := 0; i < count; i++ {
		tip := f.RequireTipSet(head)
		tips = append(tips, tip)
		head, err = tip.Parents()
		require.NoError(f.t, err)
	}
	return tips
}

// LoadMessages returns the message collections tracked by the builder.
func (f *Builder) LoadMessages(ctx context.Context, meta types.TxMeta) ([]*types.SignedMessage, []*types.UnsignedMessage, error) {
	return f.messages.LoadMessages(ctx, meta)
}

// LoadReceipts returns the message collections tracked by the builder.
func (f *Builder) LoadReceipts(ctx context.Context, c cid.Cid) ([]*types.MessageReceipt, error) {
	return f.messages.LoadReceipts(ctx, c)
}

// StoreReceipts stores message receipts and returns a commitment.
func (f *Builder) StoreReceipts(ctx context.Context, receipts []*types.MessageReceipt) (cid.Cid, error) {
	return f.messages.StoreReceipts(ctx, receipts)
}

///// Internals /////

func makeCid(i interface{}) (cid.Cid, error) {
	bytes, err := encoding.Encode(i)
	if err != nil {
		return cid.Undef, err
	}
	return cid.Prefix{
		Version:  1,
		Codec:    cid.DagCBOR,
		MhType:   types.DefaultHashFunction,
		MhLength: -1,
	}.Sum(bytes)
}
